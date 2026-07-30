import pandas as pd
import pyodbc
import os
from dotenv import load_dotenv

load_dotenv()

LOCAL_SERVER = os.getenv('LOCAL_SERVER')
LOCAL_DATABASE = os.getenv('LOCAL_DATABASE', 'local_project_db')
LOCAL_UID = os.getenv('LOCAL_UID')
LOCAL_PWD = os.getenv('LOCAL_PWD')

EXCEL_FILE = input("Введите путь к Excel-файлу со справочником сетей: ").strip().strip("'\"")

COLUMN_MAPPING = {
    "Сеть":               "network_name",
    "КАМ":                "kam",
    "Тип сети":           "network_type",
    "Сегмент":            "top20_segment",
    "Ключевой регион":    "key_region",
}

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

        # Переименовываем колонки
        rename_dict = {}
        for excel_col, db_col in COLUMN_MAPPING.items():
            if excel_col in df.columns:
                rename_dict[excel_col] = db_col

        df.rename(columns=rename_dict, inplace=True)
        print(f"🔄 Переименовано {len(rename_dict)} колонок")

        # Оставляем только нужные колонки
        cols = list(rename_dict.values())
        df = df[cols]

        # Чистим строки
        for col in cols:
            df[col] = df[col].apply(
                lambda x: str(x).strip() if pd.notna(x) and str(x).strip() != '' else None
            )

        conn = connect_to_db()
        if not conn:
            return

        cursor = conn.cursor()

        # Создаём таблицу если нет
        cursor.execute("""
            IF OBJECT_ID('dbo.tbl_NetworkGeoMapping', 'U') IS NULL
            CREATE TABLE dbo.tbl_NetworkGeoMapping (
                id              INT IDENTITY(1,1) PRIMARY KEY,
                network_name    NVARCHAR(256) NOT NULL UNIQUE,
                kam             NVARCHAR(128) NULL,
                network_type    NVARCHAR(128) NULL,
                top20_segment   NVARCHAR(128) NULL,
                key_region      NVARCHAR(128) NULL
            )
        """)
        conn.commit()
        print("✅ Таблица tbl_NetworkGeoMapping проверена/создана")

        # MERGE: обновить существующие, вставить новые
        merge_sql = """
            MERGE dbo.tbl_NetworkGeoMapping AS t
            USING (VALUES (?, ?, ?, ?, ?)) AS s (network_name, kam, network_type, top20_segment, key_region)
            ON t.network_name = s.network_name
            WHEN MATCHED THEN UPDATE SET
                kam = s.kam,
                network_type = s.network_type,
                top20_segment = s.top20_segment,
                key_region = s.key_region
            WHEN NOT MATCHED THEN INSERT (network_name, kam, network_type, top20_segment, key_region)
            VALUES (s.network_name, s.kam, s.network_type, s.top20_segment, s.key_region);
        """

        cursor.fast_executemany = True
        rows = [tuple(row) for _, row in df.iterrows()]
        cursor.executemany(merge_sql, rows)
        conn.commit()

        print(f"✅ Импорт завершён! Обработано {len(rows)} записей")

        cursor.close()
        conn.close()

    except Exception as e:
        print(f"❌ Ошибка: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()