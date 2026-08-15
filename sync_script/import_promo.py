import pandas as pd
import pyodbc
import os
import re
from datetime import datetime
from dotenv import load_dotenv

load_dotenv()

LOCAL_SERVER = os.getenv('LOCAL_SERVER')
LOCAL_DATABASE = os.getenv('LOCAL_DATABASE', 'local_project_db')
LOCAL_UID = os.getenv('LOCAL_UID')
LOCAL_PWD = os.getenv('LOCAL_PWD')

# Текущая реализация импорта небезопасна для данных: она отключена до перехода
# на staging-таблицу и одну атомарную MERGE-транзакцию.
if os.getenv('ALLOW_UNSAFE_PROMO_IMPORT') != 'I_UNDERSTAND_THIS_IMPORT_IS_UNSAFE':
    raise SystemExit(
        'Импорт промо временно отключён: текущий алгоритм может удалить активные записи. '
        'Используйте восстановленную безопасную версию со staging-таблицей.'
    )

EXCEL_FILE = input("Введите путь к Excel-файлу с данными промо: ").strip().strip("'\"")

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
        return int(float(str(value).replace(',', '.').strip()))
    except (ValueError, TypeError):
        return None

def safe_float(value):
    if pd.isna(value) or value == '' or value is None:
        return None
    try:
        return float(str(value).replace(',', '.').replace(' ', '').strip())
    except (ValueError, TypeError):
        return None

def safe_date(value):
    if pd.isna(value) or value is None or value == '':
        return None
    try:
        if isinstance(value, datetime):
            return value
        return pd.to_datetime(value).to_pydatetime()
    except:
        return None

def convert_month(value):
    if pd.isna(value) or value is None or value == '':
        return None
    try:
        return int(float(str(value).strip()))
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

def connect_to_db():
    try:
        drivers = ['ODBC Driver 18 for SQL Server', 'ODBC Driver 17 for SQL Server']
        for driver in drivers:
            try:
                conn_str = f'DRIVER={{{driver}}};SERVER={LOCAL_SERVER};DATABASE={LOCAL_DATABASE};UID={LOCAL_UID};PWD={LOCAL_PWD};TrustServerCertificate=yes;'
                conn = pyodbc.connect(conn_str)
                print(f"✅ Подключение к {LOCAL_DATABASE} через {driver}")
                return conn
            except pyodbc.Error:
                continue
        raise Exception("Не найден ODBC драйвер")
    except Exception as e:
        print(f"❌ Ошибка подключения: {e}")
        return None

def main():
    if not os.path.exists(EXCEL_FILE):
        print(f"❌ Файл не найден: {EXCEL_FILE}")
        return

    print(f"📂 Читаю файл: {EXCEL_FILE}")

    try:
        df = pd.read_excel(EXCEL_FILE, dtype=str)
        print(f"✅ Загружено {len(df)} строк, {len(df.columns)} колонок")

        rename_dict = {}
        for excel_col, db_col in COLUMN_MAPPING.items():
            if excel_col in df.columns:
                rename_dict[excel_col] = db_col

        df.rename(columns=rename_dict, inplace=True)
        print(f"🔄 Переименовано {len(rename_dict)} колонок")

        # Конвертация квартала: Q1 → 1
        if 'quarter' in df.columns:
            before = df['quarter'].iloc[0] if len(df) > 0 else 'N/A'
            df['quarter'] = df['quarter'].apply(convert_quarter)
            print(f"🔄 Кварталы сконвертированы (было: {before})")

        conn = connect_to_db()
        if not conn:
            return

        cursor = conn.cursor()

        # Добавляем колонку deleted_at если её ещё нет
        cursor.execute("""
            IF COL_LENGTH('dbo.tbl_PromoActivities', 'deleted_at') IS NULL
            ALTER TABLE dbo.tbl_PromoActivities ADD deleted_at DATETIME NULL
        """)
        conn.commit()

        # Помечаем ВСЕ активные записи как удалённые. MERGE восстановит те, что есть в Excel.
        cursor.execute("UPDATE dbo.tbl_PromoActivities SET deleted_at = GETDATE() WHERE deleted_at IS NULL")
        marked = cursor.rowcount
        print(f"🏷️ Помечено как удалённые: {marked} записей")

        db_columns = [col for col in rename_dict.values() if col in df.columns]

        # Строим MERGE
        update_clause = ', '.join([f"t.[{col}] = s.[{col}]" for col in db_columns])
        insert_cols = ', '.join([f"[{col}]" for col in db_columns])
        insert_vals = ', '.join([f"s.[{col}]" for col in db_columns])

        merge_sql = f"""
            MERGE dbo.tbl_PromoActivities AS t
            USING (VALUES ({{placeholders}})) AS s ({insert_cols})
            ON t.sku = s.sku AND t.network_name = s.network_name 
               AND t.[year] = s.[year] AND t.[month] = s.[month] 
               AND t.mechanics = s.mechanics AND t.deleted_at IS NULL
            WHEN MATCHED THEN UPDATE SET {update_clause}
            WHEN NOT MATCHED BY TARGET THEN INSERT ({insert_cols}) VALUES ({insert_vals})
            WHEN NOT MATCHED BY SOURCE AND t.deleted_at IS NULL THEN UPDATE SET t.deleted_at = GETDATE();
        """

        # Конвертируем данные
        rows_to_insert = []
        errors = []

        for idx, (_, row) in enumerate(df.iterrows()):
            try:
                values = []
                for col in db_columns:
                    val = convert_value(col, row[col])
                    values.append(val)
                rows_to_insert.append(tuple(values))
            except Exception as e:
                errors.append(f"Строка {idx+2}: {str(e)[:100]}")

        if errors:
            print(f"⚠️ Ошибок конвертации: {len(errors)} из {len(df)} строк")

        # Выполняем MERGE батчами
        batch_size = 500
        cursor.fast_executemany = True
        total_processed = 0

        # Формируем плейсхолдеры для одного батча
        single_row = '(' + ', '.join(['?' for _ in db_columns]) + ')'

        for i in range(0, len(rows_to_insert), batch_size):
            batch = rows_to_insert[i:i+batch_size]
            placeholders_str = ', '.join([single_row for _ in batch])
            flat_values = [val for row in batch for val in row]

            batch_sql = merge_sql.replace('{placeholders}', placeholders_str)

            try:
                cursor.execute(batch_sql, flat_values)
                conn.commit()
                total_processed += len(batch)
                print(f"  ✓ {total_processed}/{len(rows_to_insert)}")
            except Exception as e:
                print(f"  ❌ Ошибка в батче {i//batch_size + 1}: {str(e)[:200]}")
                # Пробуем по одной
                for row_data in batch:
                    try:
                        single_merge = merge_sql.replace('{placeholders}', single_row)
                        cursor.execute(single_merge, row_data)
                        conn.commit()
                        total_processed += 1
                    except Exception as e2:
                        pass

        # Окончательно помечаем удалёнными записи, которых не было в Excel
        # (те, что остались deleted_at IS NULL после MERGE WHEN NOT MATCHED BY SOURCE)
        cursor.execute("UPDATE dbo.tbl_PromoActivities SET deleted_at = GETDATE() WHERE deleted_at IS NULL")
        remaining = cursor.rowcount
        if remaining > 0:
            print(f"  🗑️ Помечено удалёнными: {remaining} записей (отсутствуют в Excel)")

        print(f"✅ Импорт завершён! Обработано {total_processed} из {len(rows_to_insert)} записей")

        cursor.close()
        conn.close()

    except Exception as e:
        print(f"❌ Ошибка: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()
