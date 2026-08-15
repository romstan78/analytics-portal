import argparse
import hashlib
import json
import sys
import uuid
from collections import Counter
from datetime import date, datetime, timezone
from decimal import Decimal
from pathlib import Path

from import_promo import (
    BUSINESS_KEY,
    LOCAL_DATABASE,
    TABLE_NAME,
    connect_to_db,
    quote_identifier,
)


PLAN_VERSION = 1
CONFIRMATION_TOKEN = 'APPLY_SAFE_EXACT_DUPLICATES'
ROLLBACK_CONFIRMATION_TOKEN = 'ROLLBACK_SAFE_DEDUP'
METADATA_COLUMNS = {
    'id', 'created_at', 'updated_at', 'created_by', 'updated_by', 'deleted_at'
}
APPROVAL_COLUMNS = {
    'agreement1', 'agreement2',
    'agreement1_status', 'agreement1_comment',
    'agreement2_status', 'agreement2_comment',
}
RELATED_COUNT_COLUMNS = {'comment_count', 'audit_count'}


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description='Аудит и безопасное объединение точных дублей промо.'
    )
    commands = parser.add_subparsers(dest='command', required=True)

    report = commands.add_parser('report', help='Только чтение: создать JSON-план')
    report.add_argument('--output', required=True, type=Path, help='Путь к JSON-плану')

    apply_command = commands.add_parser(
        'apply', help='Применить только exact_duplicate из ранее созданного плана'
    )
    apply_command.add_argument('--plan', required=True, type=Path, help='Путь к JSON-плану')
    apply_command.add_argument('--confirm', required=True, help='Защитная фраза подтверждения')

    rollback = commands.add_parser('rollback', help='Откатить ранее применённый run_id')
    rollback.add_argument('--run-id', required=True, help='UUID запуска очистки')
    rollback.add_argument('--confirm', required=True, help='Защитная фраза подтверждения')
    return parser.parse_args(argv)


def json_value(value):
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    if isinstance(value, Decimal):
        return format(value, 'f')
    if isinstance(value, bytes):
        return value.hex()
    return value


def canonical_json(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'))


def stable_hash(value):
    return hashlib.sha256(canonical_json(value).encode('utf-8')).hexdigest()


def normalize_key_value(column, value):
    if isinstance(value, str) and column in {'sku', 'network_name', 'mechanics'}:
        return value.strip().casefold()
    return json_value(value)


def group_identity(row):
    key = {
        column: normalize_key_value(column, row[column])
        for column in BUSINESS_KEY
    }
    return stable_hash(key), key


def row_fingerprint(row, table_columns):
    snapshot = {column: json_value(row.get(column)) for column in table_columns}
    return stable_hash(snapshot)


def differing_columns(rows, columns):
    different = []
    for column in columns:
        values = {canonical_json(json_value(row.get(column))) for row in rows}
        if len(values) > 1:
            different.append(column)
    return sorted(different)


def keeper_score(row, business_columns):
    approval_rank = {'approved': 4, 'rejected': 3, 'commented': 2, 'pending': 1, None: 0}
    approvals = sum(
        approval_rank.get(row.get(column), 0)
        for column in ('agreement1_status', 'agreement2_status')
    )
    populated = sum(row.get(column) is not None for column in business_columns)
    history = int(row.get('comment_count') or 0) + int(row.get('audit_count') or 0)
    updated = json_value(row.get('updated_at')) or ''
    return approvals, populated, history, updated, int(row['id'])


def analyze_rows(rows, table_columns):
    business_columns = [
        column for column in table_columns
        if column not in METADATA_COLUMNS and column not in APPROVAL_COLUMNS
    ]
    approval_columns = [column for column in table_columns if column in APPROVAL_COLUMNS]
    groups = {}

    for row in rows:
        group_id, normalized_key = group_identity(row)
        original_key = {column: json_value(row[column]) for column in BUSINESS_KEY}
        entry = groups.setdefault(
            group_id,
            {'key': original_key, 'normalized_key': normalized_key, 'rows': []},
        )
        entry['rows'].append(row)

    result = []
    for group_id, entry in sorted(groups.items()):
        members = sorted(entry['rows'], key=lambda item: int(item['id']))
        business_differences = differing_columns(members, business_columns)
        approval_differences = differing_columns(members, approval_columns)

        if business_differences:
            classification = 'data_conflict'
        elif approval_differences:
            classification = 'approval_conflict'
        else:
            classification = 'exact_duplicate'

        keeper = max(members, key=lambda row: keeper_score(row, business_columns))
        member_ids = [int(row['id']) for row in members]
        result.append({
            'group_id': group_id,
            'key': entry['key'],
            'classification': classification,
            'keeper_id': int(keeper['id']),
            'duplicate_ids': [row_id for row_id in member_ids if row_id != int(keeper['id'])],
            'member_ids': member_ids,
            'business_diff_columns': business_differences,
            'approval_diff_columns': approval_differences,
            'comment_count': sum(int(row.get('comment_count') or 0) for row in members),
            'audit_count': sum(int(row.get('audit_count') or 0) for row in members),
            'row_fingerprints': {
                str(row['id']): row_fingerprint(row, table_columns) for row in members
            },
            'safe_to_apply': classification == 'exact_duplicate',
        })
    return result


def fetch_table_columns(cursor):
    cursor.execute("""
        SELECT [name]
        FROM sys.columns
        WHERE [object_id] = OBJECT_ID('dbo.tbl_PromoActivities')
        ORDER BY [column_id]
    """)
    columns = [row[0] for row in cursor.fetchall()]
    if not columns:
        raise RuntimeError(f'Таблица {TABLE_NAME} не существует.')
    missing = sorted((set(BUSINESS_KEY) | {'id', 'deleted_at'}) - set(columns))
    if missing:
        raise RuntimeError('В таблице отсутствуют обязательные колонки: ' + ', '.join(missing))
    return columns


def fetch_duplicate_rows(cursor, table_columns):
    selected = ', '.join(f'p.{quote_identifier(column)}' for column in table_columns)
    key_columns = ', '.join(quote_identifier(column) for column in BUSINESS_KEY)
    not_null = ' AND '.join(
        f'{quote_identifier(column)} IS NOT NULL' for column in BUSINESS_KEY
    )
    cursor.execute(f"""
        WITH active_rows AS (
            SELECT *, COUNT(*) OVER (PARTITION BY {key_columns}) AS duplicate_count
            FROM {TABLE_NAME}
            WHERE [deleted_at] IS NULL AND {not_null}
        )
        SELECT {selected},
               (SELECT COUNT(*) FROM dbo.tbl_PromoComments c WHERE c.promo_id = p.id) AS comment_count,
               (SELECT COUNT(*) FROM dbo.tbl_AuditLog a WHERE a.entity_type = 'promo' AND a.entity_id = p.id) AS audit_count
        FROM active_rows p
        WHERE p.duplicate_count > 1
        ORDER BY p.[year], p.[month], p.network_name, p.sku, p.mechanics, p.id
    """)
    names = [column[0] for column in cursor.description]
    return [dict(zip(names, row)) for row in cursor.fetchall()]


def build_plan(connection):
    cursor = connection.cursor()
    try:
        table_columns = fetch_table_columns(cursor)
        rows = fetch_duplicate_rows(cursor, table_columns)
        groups = analyze_rows(rows, table_columns)
        counts = Counter(group['classification'] for group in groups)
        plan = {
            'version': PLAN_VERSION,
            'database': LOCAL_DATABASE,
            'generated_at': datetime.now(timezone.utc).isoformat(),
            'table_columns': table_columns,
            'summary': {
                'groups': len(groups),
                'excess_rows': sum(len(group['duplicate_ids']) for group in groups),
                'exact_duplicate_groups': counts['exact_duplicate'],
                'approval_conflict_groups': counts['approval_conflict'],
                'data_conflict_groups': counts['data_conflict'],
                'safe_rows_to_soft_delete': sum(
                    len(group['duplicate_ids'])
                    for group in groups if group['safe_to_apply']
                ),
            },
            'groups': groups,
        }
        plan['plan_hash'] = stable_hash(plan)
        return plan
    finally:
        cursor.close()


def validate_plan(plan):
    if plan.get('version') != PLAN_VERSION:
        raise ValueError(f'Неподдерживаемая версия плана: {plan.get("version")}')
    if plan.get('database') != LOCAL_DATABASE:
        raise ValueError(
            f'План создан для базы {plan.get("database")}, текущая база — {LOCAL_DATABASE}.'
        )
    supplied_hash = plan.get('plan_hash')
    unsigned = {key: value for key, value in plan.items() if key != 'plan_hash'}
    if supplied_hash != stable_hash(unsigned):
        raise ValueError('Контрольная сумма плана не совпадает: файл был изменён.')

    unsafe = [group['group_id'] for group in plan.get('groups', []) if group.get('safe_to_apply') and group.get('classification') != 'exact_duplicate']
    if unsafe:
        raise ValueError('План содержит небезопасные группы, ошибочно отмеченные для применения.')


def ensure_ledger(cursor):
    required_tables = {
        'tbl_PromoDedupRuns',
        'tbl_PromoDedupChanges',
        'tbl_PromoDedupRelatedMoves',
    }
    cursor.execute("""
        SELECT [name]
        FROM sys.tables
        WHERE [schema_id] = SCHEMA_ID('dbo')
          AND [name] IN ('tbl_PromoDedupRuns', 'tbl_PromoDedupChanges', 'tbl_PromoDedupRelatedMoves')
    """)
    available = {row[0] for row in cursor.fetchall()}
    missing = sorted(required_tables - available)
    if missing:
        raise RuntimeError(
            'Не применена миграция журнала очистки: ' + ', '.join(missing)
        )


def key_predicate(key):
    clauses = []
    values = []
    for column in BUSINESS_KEY:
        value = key[column]
        if value is None:
            clauses.append(f'{quote_identifier(column)} IS NULL')
        else:
            clauses.append(f'{quote_identifier(column)} = ?')
            values.append(value)
    return ' AND '.join(clauses), values


def fetch_locked_group(cursor, group, table_columns):
    selected = ', '.join(quote_identifier(column) for column in table_columns)
    predicate, values = key_predicate(group['key'])
    cursor.execute(f"""
        SELECT {selected}
        FROM {TABLE_NAME} WITH (UPDLOCK, HOLDLOCK)
        WHERE [deleted_at] IS NULL AND {predicate}
        ORDER BY id
    """, values)
    names = [column[0] for column in cursor.description]
    return [dict(zip(names, row)) for row in cursor.fetchall()]


def apply_safe_groups(connection, plan):
    table_columns = plan['table_columns']
    safe_groups = [group for group in plan['groups'] if group.get('safe_to_apply')]
    cursor = connection.cursor()
    merged_groups = 0
    soft_deleted_rows = 0
    moved_comments = 0
    moved_audit_rows = 0
    run_id = str(uuid.uuid4())

    try:
        cursor.execute('SET XACT_ABORT ON; SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;')
        ensure_ledger(cursor)
        current_columns = fetch_table_columns(cursor)
        if current_columns != table_columns:
            raise RuntimeError('Схема таблицы изменилась после формирования плана.')
        if not safe_groups:
            raise RuntimeError('В плане нет точных дублей для безопасного применения.')

        cursor.execute("""
            INSERT INTO dbo.tbl_PromoDedupRuns
                (run_id, plan_hash, status, executed_by)
            VALUES (?, ?, 'RUNNING', N'promo-dedup-tool')
        """, run_id, plan['plan_hash'])

        for group in safe_groups:
            current_rows = fetch_locked_group(cursor, group, table_columns)
            current_ids = [int(row['id']) for row in current_rows]
            if current_ids != group['member_ids']:
                raise RuntimeError(
                    f'Состав группы {group["group_id"]} изменился; применение отменено.'
                )
            for row in current_rows:
                expected = group['row_fingerprints'].get(str(row['id']))
                if row_fingerprint(row, table_columns) != expected:
                    raise RuntimeError(
                        f'Запись {row["id"]} изменилась после формирования плана.'
                    )

            duplicate_ids = group['duplicate_ids']
            if not duplicate_ids:
                continue
            placeholders = ', '.join('?' for _ in duplicate_ids)

            rows_by_id = {int(row['id']): row for row in current_rows}
            for duplicate_id in duplicate_ids:
                duplicate = rows_by_id[duplicate_id]
                cursor.execute("""
                    INSERT INTO dbo.tbl_PromoDedupChanges
                        (run_id, group_id, keeper_id, duplicate_id,
                         original_deleted_at, original_updated_at, original_updated_by)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                    run_id,
                    group['group_id'],
                    group['keeper_id'],
                    duplicate_id,
                    duplicate.get('deleted_at'),
                    duplicate.get('updated_at'),
                    duplicate.get('updated_by'),
                )

            cursor.execute(f"""
                INSERT INTO dbo.tbl_PromoDedupRelatedMoves
                    (run_id, related_table, related_id, from_promo_id, to_promo_id)
                SELECT ?, 'tbl_PromoComments', id, promo_id, ?
                FROM dbo.tbl_PromoComments
                WHERE promo_id IN ({placeholders})
            """, [run_id, group['keeper_id'], *duplicate_ids])

            cursor.execute(f"""
                INSERT INTO dbo.tbl_PromoDedupRelatedMoves
                    (run_id, related_table, related_id, from_promo_id, to_promo_id)
                SELECT ?, 'tbl_AuditLog', id, entity_id, ?
                FROM dbo.tbl_AuditLog
                WHERE entity_type = 'promo' AND entity_id IN ({placeholders})
            """, [run_id, group['keeper_id'], *duplicate_ids])

            cursor.execute(
                f'UPDATE dbo.tbl_PromoComments SET promo_id = ? WHERE promo_id IN ({placeholders})',
                [group['keeper_id'], *duplicate_ids],
            )
            moved_comments += max(cursor.rowcount, 0)

            cursor.execute(
                f"UPDATE dbo.tbl_AuditLog SET entity_id = ? WHERE entity_type = 'promo' AND entity_id IN ({placeholders})",
                [group['keeper_id'], *duplicate_ids],
            )
            moved_audit_rows += max(cursor.rowcount, 0)

            cursor.execute(
                f"""
                    UPDATE {TABLE_NAME}
                    SET [deleted_at] = GETDATE(), [updated_at] = GETDATE(),
                        [updated_by] = N'promo-dedup-tool'
                    WHERE [deleted_at] IS NULL AND id IN ({placeholders})
                """,
                duplicate_ids,
            )
            if cursor.rowcount != len(duplicate_ids):
                raise RuntimeError(
                    f'Не удалось пометить все дубли группы {group["group_id"]}.'
                )
            soft_deleted_rows += cursor.rowcount

            audit_payload = json.dumps({
                'deduplicated_ids': duplicate_ids,
                'plan_hash': plan['plan_hash'],
            }, ensure_ascii=False)
            cursor.execute("""
                INSERT INTO dbo.tbl_AuditLog
                    (entity_type, entity_id, user_name, action_type, changed_fields)
                VALUES ('promo', ?, N'promo-dedup-tool', 'DEDUPLICATE', ?)
            """, group['keeper_id'], audit_payload)
            merged_groups += 1

        result = {
            'run_id': run_id,
            'merged_groups': merged_groups,
            'soft_deleted_rows': soft_deleted_rows,
            'moved_comments': moved_comments,
            'moved_audit_rows': moved_audit_rows,
        }
        cursor.execute("""
            UPDATE dbo.tbl_PromoDedupRuns
            SET status = 'APPLIED', completed_at = SYSUTCDATETIME(), stats_json = ?
            WHERE run_id = ? AND status = 'RUNNING'
        """, json.dumps(result, ensure_ascii=False), run_id)
        connection.commit()
        return result
    except Exception:
        connection.rollback()
        raise
    finally:
        cursor.close()


def rollback_run(connection, run_id):
    cursor = connection.cursor()
    try:
        cursor.execute('SET XACT_ABORT ON; SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;')
        ensure_ledger(cursor)
        cursor.execute("""
            SELECT status
            FROM dbo.tbl_PromoDedupRuns WITH (UPDLOCK, HOLDLOCK)
            WHERE run_id = ?
        """, run_id)
        row = cursor.fetchone()
        if not row:
            raise RuntimeError(f'Запуск {run_id} не найден.')
        if row[0] != 'APPLIED':
            raise RuntimeError(f'Запуск {run_id} имеет статус {row[0]}, откат невозможен.')

        cursor.execute("""
            SELECT COUNT(*)
            FROM dbo.tbl_PromoDedupChanges c
            JOIN dbo.tbl_PromoActivities p ON p.id = c.duplicate_id
            WHERE c.run_id = ?
              AND (p.deleted_at IS NULL OR ISNULL(p.updated_by, N'') <> N'promo-dedup-tool')
        """, run_id)
        changed_duplicates = cursor.fetchone()[0]
        if changed_duplicates:
            raise RuntimeError(
                'Некоторые скрытые дубли изменились после очистки; автоматический откат отменён.'
            )

        cursor.execute("""
            SELECT COUNT(*)
            FROM dbo.tbl_PromoDedupRelatedMoves m
            LEFT JOIN dbo.tbl_PromoComments c
              ON m.related_table = 'tbl_PromoComments' AND c.id = m.related_id
            LEFT JOIN dbo.tbl_AuditLog a
              ON m.related_table = 'tbl_AuditLog' AND a.id = m.related_id
            WHERE m.run_id = ?
              AND ((m.related_table = 'tbl_PromoComments' AND (c.id IS NULL OR c.promo_id <> m.to_promo_id))
                OR (m.related_table = 'tbl_AuditLog' AND (a.id IS NULL OR a.entity_id <> m.to_promo_id)))
        """, run_id)
        changed_related = cursor.fetchone()[0]
        if changed_related:
            raise RuntimeError(
                'Связанные комментарии или аудит изменились после очистки; автоматический откат отменён.'
            )

        cursor.execute("""
            UPDATE c
            SET c.promo_id = m.from_promo_id
            FROM dbo.tbl_PromoComments c
            JOIN dbo.tbl_PromoDedupRelatedMoves m
              ON m.related_table = 'tbl_PromoComments' AND m.related_id = c.id
            WHERE m.run_id = ?
        """, run_id)
        restored_comments = max(cursor.rowcount, 0)

        cursor.execute("""
            UPDATE a
            SET a.entity_id = m.from_promo_id
            FROM dbo.tbl_AuditLog a
            JOIN dbo.tbl_PromoDedupRelatedMoves m
              ON m.related_table = 'tbl_AuditLog' AND m.related_id = a.id
            WHERE m.run_id = ?
        """, run_id)
        restored_audit_rows = max(cursor.rowcount, 0)

        cursor.execute("""
            UPDATE p
            SET p.deleted_at = c.original_deleted_at,
                p.updated_at = c.original_updated_at,
                p.updated_by = c.original_updated_by
            FROM dbo.tbl_PromoActivities p
            JOIN dbo.tbl_PromoDedupChanges c ON c.duplicate_id = p.id
            WHERE c.run_id = ?
        """, run_id)
        restored_rows = max(cursor.rowcount, 0)

        cursor.execute("""
            INSERT INTO dbo.tbl_AuditLog
                (entity_type, entity_id, user_name, action_type, changed_fields)
            SELECT DISTINCT 'promo', keeper_id, N'promo-dedup-tool', 'ROLLBACK_DEDUP', ?
            FROM dbo.tbl_PromoDedupChanges
            WHERE run_id = ?
        """, json.dumps({'rolled_back_run_id': run_id}), run_id)

        result = {
            'run_id': run_id,
            'restored_rows': restored_rows,
            'restored_comments': restored_comments,
            'restored_audit_rows': restored_audit_rows,
        }
        cursor.execute("""
            UPDATE dbo.tbl_PromoDedupRuns
            SET status = 'ROLLED_BACK', rolled_back_at = SYSUTCDATETIME(), stats_json = ?
            WHERE run_id = ? AND status = 'APPLIED'
        """, json.dumps(result, ensure_ascii=False), run_id)
        connection.commit()
        return result
    except Exception:
        connection.rollback()
        raise
    finally:
        cursor.close()


def write_plan(path, plan):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(plan, ensure_ascii=False, indent=2), encoding='utf-8')


def main(argv=None):
    args = parse_args(argv)
    try:
        if args.command == 'report':
            connection = connect_to_db()
            try:
                plan = build_plan(connection)
            finally:
                connection.close()
            write_plan(args.output, plan)
            print(json.dumps(plan['summary'], ensure_ascii=False, indent=2))
            print(f'✅ План только для чтения сохранён: {args.output}')
            return 0

        if args.command == 'apply':
            if args.confirm != CONFIRMATION_TOKEN:
                raise ValueError(
                    f'Неверная защитная фраза. Требуется: {CONFIRMATION_TOKEN}'
                )
            plan = json.loads(args.plan.read_text(encoding='utf-8'))
            validate_plan(plan)
            connection = connect_to_db()
            try:
                result = apply_safe_groups(connection, plan)
            finally:
                connection.close()
            print(json.dumps(result, ensure_ascii=False, indent=2))
            print('✅ Точные дубли объединены одной транзакцией.')
            return 0

        if args.confirm != ROLLBACK_CONFIRMATION_TOKEN:
            raise ValueError(
                f'Неверная защитная фраза. Требуется: {ROLLBACK_CONFIRMATION_TOKEN}'
            )
        run_id = str(uuid.UUID(args.run_id))
        connection = connect_to_db()
        try:
            result = rollback_run(connection, run_id)
        finally:
            connection.close()
        print(json.dumps(result, ensure_ascii=False, indent=2))
        print('✅ Очистка полностью откачена по журналу.')
        return 0
    except Exception as error:
        print(f'❌ Операция отменена: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
