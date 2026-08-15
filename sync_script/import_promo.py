import argparse
import math
import pandas as pd
import pyodbc
import os
import re
import sys
from datetime import datetime
from pathlib import Path
from dotenv import load_dotenv

load_dotenv()

LOCAL_SERVER = os.getenv('LOCAL_SERVER')
LOCAL_DATABASE = os.getenv('LOCAL_DATABASE', 'local_project_db')
LOCAL_UID = os.getenv('LOCAL_UID')
LOCAL_PWD = os.getenv('LOCAL_PWD')

COLUMN_MAPPING = {
    "Название сети": "network_name",
    "KAM": "kam",
    "ID Директум": "id_directum",
    "№ ДС": "ds_number",
    "Год": "year",
    "Месяц": "month",
    "Квартал": "quarter",
    "SKU": "sku",
    "Brand": "brand",
    "бренд для АС": "brand_as",
    "Механика/статья затрат": "mechanics",
    "сумма скидки, либо сумма баллов. С налогами!": "discount_amount",
    "GTN/OPEX": "gtn_opex",
    "Условия (писать развернуто):": "conditions",
    "Baseline уп": "baseline_units",
    "План: Promo уп": "plan_promo_units",
    "План: Инвестиции, руб": "plan_investments_rub",
    "Комментарии": "comments",
    "Динамика категории по брендам": "category_dynamics",
    "Фактический скорректированный Baseline": "actual_corrected_baseline",
    "Факт: Продажи по отчетам сетей (уп)": "actual_network_sales_units",
    "Факт: продажи по промо (уп)": "actual_promo_sales_units",
    "Факт: Инвестиции": "actual_investments",
    "Е-ком сегмент": "ecom_segment",
    "Факт: внешний е-ком (уп)": "actual_external_ecom_units",
    "Статус Акции": "status",
    "Borzenkov A": "agreement1",
    "Sapunova N.": "agreement2",
    "Baseline руб": "baseline_rub",
    "План: Promo руб": "plan_promo_rub",
    "План: Promo Uplift уп": "plan_promo_uplift_units",
    "План: Promo Uplift % (уп)": "plan_promo_uplift_pct_units",
    "План: Promo Uplift руб": "plan_promo_uplift_rub",
    "План: Promo Uplift % (руб)": "plan_promo_uplift_pct_rub",
    "План: % Инвестиций": "plan_investments_pct",
    "План: ROI ": "plan_roi",
    "максимальные продажи уп": "max_sales_units",
    "Цена контракта": "contract_price",
    "GM": "gm",
    "Кол-во аптек в сети ТОТАЛ": "total_pharmacies",
    "Кол-во аптек в промо": "promo_pharmacies",
    "Факт: Promo Руб": "actual_promo_rub",
    "Факт: Promo Uplift (уп)": "actual_promo_uplift_units",
    "Факт: Promo Uplift (руб)": "actual_promo_uplift_rub",
    "Promo Uplift чистый (с учетом GM), руб": "net_promo_uplift_rub",
    "Факт: Чистый Promo Uplift (%)": "net_promo_uplift_pct",
    "Факт: % Инвестиций": "actual_investments_pct",
    "Факт: ROI ": "actual_roi",
    "Факт: Promo Руб w/o ecom": "actual_promo_rub_wo_ecom",
    "Факт: Promo Uplift (уп) w/o ecom": "actual_promo_uplift_units_wo_ecom",
    "Факт: Promo Uplift (руб) w/o ecom": "actual_promo_uplift_rub_wo_ecom",
    "Promo Uplift чистый (с учетом GM), w/o ecom": "net_promo_uplift_rub_wo_ecom",
    "Факт: Чистый Promo Uplift (%) w/o ecom": "net_promo_uplift_pct_wo_ecom",
    "Факт: % Инвестиций w/o ecom": "actual_investments_pct_wo_ecom",
    "Факт: ROI w/o ecom": "actual_roi_wo_ecom",
    "План vs Факт (руб)": "plan_vs_fact_rub",
    "План vs Факт (инвестиции)": "plan_vs_fact_investments",
    "Регион ключевого влияния": "key_region",
    "Оборачиваемость на точку": "turnover_per_point",
    "Оборачиваемость на точку в промо": "turnover_per_point_promo",
    "Сегмент ТОП-20": "top20_segment",
    "цена OLAP": "olap_price",
    "план промо СИП OLAP": "plan_promo_cip_olap",
    "факт промо СИП OLAP": "fact_promo_cip_olap",
    "план Promo Uplift СИП OLAP": "plan_promo_uplift_cip_olap",
    "факт Promo Uplift СИП OLAP": "fact_promo_uplift_cip_olap",
    "Дата": "date",
}

INT_FIELDS = ['year', 'quarter', 'total_pharmacies', 'promo_pharmacies']

FLOAT_FIELDS = [
    'discount_amount', 'baseline_units', 'plan_promo_units', 'plan_investments_rub',
    'category_dynamics', 'actual_corrected_baseline', 'actual_network_sales_units',
    'actual_promo_sales_units', 'actual_investments', 'actual_external_ecom_units',
    'baseline_rub', 'plan_promo_rub', 'plan_promo_uplift_units', 'plan_promo_uplift_pct_units',
    'plan_promo_uplift_rub', 'plan_promo_uplift_pct_rub', 'plan_investments_pct', 'plan_roi',
    'max_sales_units', 'contract_price', 'gm', 'actual_promo_rub', 'actual_promo_uplift_units',
    'actual_promo_uplift_rub', 'net_promo_uplift_rub', 'net_promo_uplift_pct',
    'actual_investments_pct', 'actual_roi', 'actual_promo_rub_wo_ecom',
    'actual_promo_uplift_units_wo_ecom', 'actual_promo_uplift_rub_wo_ecom',
    'net_promo_uplift_rub_wo_ecom', 'net_promo_uplift_pct_wo_ecom',
    'actual_investments_pct_wo_ecom', 'actual_roi_wo_ecom', 'plan_vs_fact_rub',
    'plan_vs_fact_investments', 'turnover_per_point', 'turnover_per_point_promo',
    'olap_price', 'plan_promo_cip_olap', 'fact_promo_cip_olap',
    'plan_promo_uplift_cip_olap', 'fact_promo_uplift_cip_olap',
]

AGREEMENT_FIELDS = ['agreement1', 'agreement2']

BUSINESS_KEY = ['sku', 'network_name', 'year', 'month', 'mechanics']
TABLE_NAME = 'dbo.tbl_PromoActivities'

MONTH_NAMES = {
    'январь': 1, 'февраль': 2, 'март': 3, 'апрель': 4,
    'май': 5, 'июнь': 6, 'июль': 7, 'август': 8,
    'сентябрь': 9, 'октябрь': 10, 'ноябрь': 11, 'декабрь': 12,
    'january': 1, 'february': 2, 'march': 3, 'april': 4,
    'may': 5, 'june': 6, 'july': 7, 'august': 8,
    'september': 9, 'october': 10, 'november': 11, 'december': 12,
}

def clean_string(value):
    if value is None or pd.isna(value):
        return None
    s = str(value)
    s = re.sub(r'[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]', '', s)
    s = s.replace('\u00a0', ' ').replace('\u200b', '').replace('\ufeff', '')
    s = s.strip()
    return s if s else None

def safe_int(value):
    if pd.isna(value) or value == '' or value is None:
        return None
    try:
        parsed = float(str(value).replace(',', '.').strip())
        return int(parsed) if math.isfinite(parsed) and parsed.is_integer() else None
    except (ValueError, TypeError):
        return None

def safe_float(value):
    if pd.isna(value) or value == '' or value is None:
        return None
    try:
        parsed = float(str(value).replace(',', '.').replace(' ', '').strip())
        return parsed if math.isfinite(parsed) else None
    except (ValueError, TypeError):
        return None

def safe_date(value):
    if pd.isna(value) or value is None or value == '':
        return None
    try:
        if isinstance(value, datetime):
            return value
        return pd.to_datetime(value).to_pydatetime()
    except (ValueError, TypeError, OverflowError):
        return None

def convert_month(value):
    if pd.isna(value) or value is None or value == '':
        return None
    try:
        parsed = float(str(value).strip())
        return int(parsed) if math.isfinite(parsed) and parsed.is_integer() else None
    except (ValueError, TypeError):
        month_str = clean_string(value)
        if month_str and month_str.lower() in MONTH_NAMES:
            return MONTH_NAMES[month_str.lower()]
        if month_str:
            for name, num in MONTH_NAMES.items():
                if name.startswith(month_str.lower()[:3]):
                    return num
        return None

def convert_value(col_name, value):
    if col_name == 'month':
        return convert_month(value)
    elif col_name in INT_FIELDS:
        return safe_int(value)
    elif col_name in FLOAT_FIELDS:
        return safe_float(value)
    elif col_name == 'date':
        return safe_date(value)
    elif col_name in ['agreement1', 'agreement2']:
        # Согласование: '0' → None, иначе чистый текст
        cleaned = clean_string(value)
        if cleaned == '0':
            return None
        return cleaned
    else:
        return clean_string(value)

def convert_quarter(val):
    if pd.isna(val) or val == '' or val is None:
        return None
    s = str(val).strip()
    m = re.match(r'^[Qq]\s*(\d)$', s)
    if m:
        return int(m.group(1))
    return safe_int(val)

def is_blank(value):
    return value is None or pd.isna(value) or str(value).strip() == ''


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description='Безопасный атомарный импорт промо из Excel в SQL Server.'
    )
    parser.add_argument('excel_file', type=Path, help='Путь к Excel-файлу')
    parser.add_argument(
        '--full-snapshot',
        action='store_true',
        help='Пометить удалёнными активные записи, которых нет в файле. По умолчанию импорт только добавляет и обновляет.',
    )
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='Прочитать и полностью проверить файл без подключения к базе и без изменений.',
    )
    return parser.parse_args(argv)


def prepare_dataframe(excel_file):
    if not excel_file.is_file():
        raise ValueError(f'Файл не найден: {excel_file}')

    source = pd.read_excel(excel_file, dtype=object)
    if source.empty:
        raise ValueError('Excel-файл не содержит строк; импорт отменён.')

    rename_dict = {
        excel_col: db_col
        for excel_col, db_col in COLUMN_MAPPING.items()
        if excel_col in source.columns
    }
    mapped_columns = list(rename_dict.values())
    missing_keys = [column for column in BUSINESS_KEY if column not in mapped_columns]
    if missing_keys:
        missing_headers = [
            excel_col for excel_col, db_col in COLUMN_MAPPING.items() if db_col in missing_keys
        ]
        raise ValueError(
            'Нет обязательных колонок бизнес-ключа: ' + ', '.join(missing_headers)
        )

    source = source.rename(columns=rename_dict)[mapped_columns].copy()
    conversion_errors = []

    for column in mapped_columns:
        converted = []
        for index, value in source[column].items():
            result = convert_quarter(value) if column == 'quarter' else convert_value(column, value)
            intentional_null = (
                column in AGREEMENT_FIELDS and clean_string(value) == '0'
            )
            if not is_blank(value) and result is None and not intentional_null:
                conversion_errors.append(
                    f'строка {index + 2}, колонка {column}: {value!r}'
                )
            converted.append(result)
        source[column] = converted

    if conversion_errors:
        preview = '; '.join(conversion_errors[:10])
        suffix = f'; ещё ошибок: {len(conversion_errors) - 10}' if len(conversion_errors) > 10 else ''
        raise ValueError(f'Некорректные значения: {preview}{suffix}')

    null_key_rows = source[BUSINESS_KEY].isna().any(axis=1)
    if null_key_rows.any():
        rows = ', '.join(str(index + 2) for index in source.index[null_key_rows][:10])
        raise ValueError(f'Пустые значения бизнес-ключа в строках: {rows}')

    if not source['month'].between(1, 12).all():
        raise ValueError('Месяц должен быть числом от 1 до 12.')
    if not source['year'].between(2000, 2100).all():
        raise ValueError('Год должен быть числом от 2000 до 2100.')
    if 'quarter' in source and not source['quarter'].dropna().between(1, 4).all():
        raise ValueError('Квартал должен быть числом от 1 до 4.')

    normalized_keys = source[BUSINESS_KEY].copy()
    for column in ['sku', 'network_name', 'mechanics']:
        normalized_keys[column] = normalized_keys[column].map(lambda value: value.casefold())
    duplicate_rows = normalized_keys.duplicated(keep=False)
    if duplicate_rows.any():
        rows = ', '.join(str(index + 2) for index in source.index[duplicate_rows][:10])
        raise ValueError(f'Дубли бизнес-ключа в Excel в строках: {rows}')

    return source, mapped_columns


def connect_to_db():
    missing = [
        name for name, value in {
            'LOCAL_SERVER': LOCAL_SERVER,
            'LOCAL_UID': LOCAL_UID,
            'LOCAL_PWD': LOCAL_PWD,
        }.items() if not value
    ]
    if missing:
        raise RuntimeError('Не заданы переменные окружения: ' + ', '.join(missing))

    errors = []
    for driver in ['ODBC Driver 18 for SQL Server', 'ODBC Driver 17 for SQL Server']:
        try:
            conn_str = (
                f'DRIVER={{{driver}}};SERVER={LOCAL_SERVER};DATABASE={LOCAL_DATABASE};'
                f'UID={LOCAL_UID};PWD={LOCAL_PWD};TrustServerCertificate=yes;'
            )
            connection = pyodbc.connect(conn_str, autocommit=False)
            print(f'✅ Подключение к {LOCAL_DATABASE} через {driver}')
            return connection
        except pyodbc.Error as error:
            errors.append(f'{driver}: {error.args[0] if error.args else "ошибка подключения"}')

    raise RuntimeError('Не удалось подключиться к SQL Server. ' + '; '.join(errors))


def quote_identifier(identifier):
    if not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', identifier):
        raise ValueError(f'Недопустимое имя SQL-колонки: {identifier}')
    return f'[{identifier}]'


def build_merge_sql(db_columns, full_snapshot=False):
    quoted_columns = ', '.join(quote_identifier(column) for column in db_columns)
    source_columns = ', '.join(f's.{quote_identifier(column)}' for column in db_columns)
    update_columns = [column for column in db_columns if column not in BUSINESS_KEY]
    assignments = [
        f't.{quote_identifier(column)} = s.{quote_identifier(column)}'
        for column in update_columns
    ]
    assignments.extend(['t.[deleted_at] = NULL', 't.[updated_at] = GETDATE()'])
    match = ' AND '.join(
        f't.{quote_identifier(column)} = s.{quote_identifier(column)}'
        for column in BUSINESS_KEY
    )
    delete_clause = ''
    if full_snapshot:
        delete_clause = """
        WHEN NOT MATCHED BY SOURCE AND t.[deleted_at] IS NULL THEN
            UPDATE SET t.[deleted_at] = GETDATE(), t.[updated_at] = GETDATE()"""

    return f"""
        DECLARE @merge_actions TABLE ([action] NVARCHAR(20));

        MERGE {TABLE_NAME} WITH (HOLDLOCK) AS t
        USING #PromoImportStage AS s
          ON {match} AND t.[deleted_at] IS NULL
        WHEN MATCHED THEN
            UPDATE SET {', '.join(assignments)}
        WHEN NOT MATCHED BY TARGET THEN
            INSERT ({quoted_columns}) VALUES ({source_columns})
        {delete_clause}
        OUTPUT CASE
            WHEN $action = 'UPDATE' AND s.[sku] IS NULL THEN 'SOFT_DELETE'
            ELSE $action
        END INTO @merge_actions;

        SELECT [action], COUNT(*) AS [count]
        FROM @merge_actions
        GROUP BY [action];
    """


def validate_destination(cursor, db_columns):
    cursor.execute("""
        SELECT [name]
        FROM sys.columns
        WHERE [object_id] = OBJECT_ID('dbo.tbl_PromoActivities')
    """)
    available_columns = {row[0] for row in cursor.fetchall()}
    if not available_columns:
        raise RuntimeError('Таблица dbo.tbl_PromoActivities не существует.')

    required = set(db_columns) | {'deleted_at', 'updated_at'}
    missing = sorted(required - available_columns)
    if missing:
        raise RuntimeError('В таблице отсутствуют колонки: ' + ', '.join(missing))

    key_list = ', '.join(quote_identifier(column) for column in BUSINESS_KEY)
    not_null = ' AND '.join(f'{quote_identifier(column)} IS NOT NULL' for column in BUSINESS_KEY)
    cursor.execute(f"""
        WITH duplicate_keys AS (
            SELECT COUNT(*) AS duplicate_count
            FROM {TABLE_NAME}
            WHERE [deleted_at] IS NULL AND {not_null}
            GROUP BY {key_list}
            HAVING COUNT(*) > 1
        )
        SELECT COUNT(*) AS duplicate_groups,
               COALESCE(SUM(duplicate_count - 1), 0) AS excess_rows
        FROM duplicate_keys
    """)
    duplicate_groups, excess_rows = cursor.fetchone()
    if duplicate_groups:
        raise RuntimeError(
            'В базе уже есть дубли активного бизнес-ключа; импорт отменён. '
            f'Групп: {duplicate_groups}, лишних строк: {excess_rows}. '
            'Сначала разберите дубли отдельной контролируемой процедурой.'
        )


def import_dataframe(connection, dataframe, db_columns, full_snapshot=False):
    cursor = connection.cursor()
    quoted_columns = ', '.join(quote_identifier(column) for column in db_columns)
    placeholders = ', '.join('?' for _ in db_columns)

    try:
        cursor.execute('SET XACT_ABORT ON; SET NOCOUNT ON;')
        validate_destination(cursor, db_columns)
        cursor.execute(f"""
            SELECT TOP (0) {quoted_columns}
            INTO #PromoImportStage
            FROM {TABLE_NAME};
        """)

        rows = list(dataframe[db_columns].itertuples(index=False, name=None))
        cursor.executemany(
            f'INSERT INTO #PromoImportStage ({quoted_columns}) VALUES ({placeholders})',
            rows,
        )
        cursor.execute(build_merge_sql(db_columns, full_snapshot=full_snapshot))
        stats = {row[0]: row[1] for row in cursor.fetchall()}
        connection.commit()
        return stats
    except Exception:
        connection.rollback()
        raise
    finally:
        cursor.close()


def main(argv=None):
    args = parse_args(argv)
    try:
        print(f'📂 Проверяю файл: {args.excel_file}')
        dataframe, db_columns = prepare_dataframe(args.excel_file)
        print(f'✅ Проверено {len(dataframe)} строк, {len(db_columns)} колонок')

        if args.dry_run:
            print('✅ Dry-run завершён: база данных не изменялась.')
            return 0

        if args.full_snapshot:
            print('⚠️ Режим полного снимка: отсутствующие в файле активные промо будут помечены удалёнными.')
        else:
            print('ℹ️ Безопасный режим: отсутствующие в файле записи не изменяются.')

        connection = connect_to_db()
        try:
            stats = import_dataframe(
                connection,
                dataframe,
                db_columns,
                full_snapshot=args.full_snapshot,
            )
        finally:
            connection.close()

        print(
            '✅ Импорт зафиксирован одной транзакцией: '
            f'добавлено {stats.get("INSERT", 0)}, '
            f'обновлено {stats.get("UPDATE", 0)}, '
            f'помечено удалёнными {stats.get("SOFT_DELETE", 0)}.'
        )
        return 0
    except Exception as error:
        print(f'❌ Импорт отменён, изменения не зафиксированы: {error}', file=sys.stderr)
        return 1

if __name__ == "__main__":
    raise SystemExit(main())
