import unittest
from datetime import datetime

import dedupe_promo


TABLE_COLUMNS = [
    'id', 'network_name', 'year', 'month', 'sku', 'mechanics',
    'conditions', 'plan_promo_units',
    'agreement1_status', 'agreement1_comment',
    'agreement2_status', 'agreement2_comment',
    'created_at', 'updated_at', 'created_by', 'updated_by', 'deleted_at',
]


def promo_row(row_id, **changes):
    row = {
        'id': row_id,
        'network_name': 'Сеть',
        'year': 2026,
        'month': 8,
        'sku': 'SKU-1',
        'mechanics': 'Скидка',
        'conditions': 'Условия',
        'plan_promo_units': 100,
        'agreement1_status': None,
        'agreement1_comment': None,
        'agreement2_status': None,
        'agreement2_comment': None,
        'created_at': datetime(2026, 8, 1, 10, 0, 0),
        'updated_at': datetime(2026, 8, 1, 10, 0, 0),
        'created_by': 'tester',
        'updated_by': 'tester',
        'deleted_at': None,
        'comment_count': 0,
        'audit_count': 0,
    }
    row.update(changes)
    return row


class AnalyzeRowsTests(unittest.TestCase):
    def test_metadata_only_difference_is_exact_duplicate(self):
        groups = dedupe_promo.analyze_rows([
            promo_row(1),
            promo_row(2, updated_at=datetime(2026, 8, 2, 10, 0, 0)),
        ], TABLE_COLUMNS)

        self.assertEqual(groups[0]['classification'], 'exact_duplicate')
        self.assertTrue(groups[0]['safe_to_apply'])
        self.assertEqual(groups[0]['duplicate_ids'], [1])

    def test_approval_difference_requires_manual_resolution(self):
        groups = dedupe_promo.analyze_rows([
            promo_row(1),
            promo_row(2, agreement1_status='approved'),
        ], TABLE_COLUMNS)

        self.assertEqual(groups[0]['classification'], 'approval_conflict')
        self.assertFalse(groups[0]['safe_to_apply'])
        self.assertEqual(groups[0]['keeper_id'], 2)

    def test_business_difference_is_not_deduplicated(self):
        groups = dedupe_promo.analyze_rows([
            promo_row(1),
            promo_row(2, conditions='Другие условия'),
        ], TABLE_COLUMNS)

        self.assertEqual(groups[0]['classification'], 'data_conflict')
        self.assertEqual(groups[0]['business_diff_columns'], ['conditions'])
        self.assertFalse(groups[0]['safe_to_apply'])

    def test_grouping_matches_case_insensitive_sql_key(self):
        groups = dedupe_promo.analyze_rows([
            promo_row(1),
            promo_row(2, sku='sku-1', network_name='сеть', mechanics='скидка'),
        ], TABLE_COLUMNS)

        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]['member_ids'], [1, 2])


class PlanValidationTests(unittest.TestCase):
    def make_plan(self):
        plan = {
            'version': dedupe_promo.PLAN_VERSION,
            'database': dedupe_promo.LOCAL_DATABASE,
            'generated_at': '2026-08-16T00:00:00+00:00',
            'table_columns': TABLE_COLUMNS,
            'summary': {},
            'groups': [],
        }
        plan['plan_hash'] = dedupe_promo.stable_hash(plan)
        return plan

    def test_accepts_unchanged_plan(self):
        dedupe_promo.validate_plan(self.make_plan())

    def test_rejects_modified_plan(self):
        plan = self.make_plan()
        plan['database'] = 'other_database'

        with self.assertRaisesRegex(ValueError, 'текущая база'):
            dedupe_promo.validate_plan(plan)


class FakeCursor:
    """Курсор, отвечающий заранее заданной очередью результатов."""

    def __init__(self, results):
        self.results = list(results)
        self.queries = []

    def execute(self, query, *args):
        self.queries.append((query, args))

    def fetchone(self):
        return self.results.pop(0)


class DedupLockTests(unittest.TestCase):
    def test_acquires_exclusive_lock_on_shared_resource(self):
        cursor = FakeCursor([(1,), (0,)])

        dedupe_promo.acquire_dedup_lock(cursor)

        lock_query = cursor.queries[1][0]
        self.assertIn('sp_getapplock', lock_query)
        self.assertIn("@LockMode = 'Exclusive'", lock_query)
        # Владелец — транзакция: блокировка снимается сама при обрыве.
        self.assertIn("@LockOwner = 'Transaction'", lock_query)
        self.assertEqual(
            cursor.queries[1][1],
            (dedupe_promo.DEDUP_LOCK_RESOURCE, dedupe_promo.DEDUP_LOCK_TIMEOUT_MS),
        )

    def test_refuses_to_run_without_transaction(self):
        cursor = FakeCursor([(0,)])

        with self.assertRaisesRegex(RuntimeError, 'открытой транзакции'):
            dedupe_promo.acquire_dedup_lock(cursor)

    def test_fails_when_backend_keeps_writing(self):
        # -1 — истёк таймаут ожидания: бэкенд держит разделяемую блокировку.
        cursor = FakeCursor([(1,), (-1,)])

        with self.assertRaisesRegex(RuntimeError, 'код -1'):
            dedupe_promo.acquire_dedup_lock(cursor)


if __name__ == '__main__':
    unittest.main()
