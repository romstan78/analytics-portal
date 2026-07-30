import pyodbc
import sys
import os
from dotenv import load_dotenv

load_dotenv()

SOURCE_SERVER = os.getenv('OLAP_SERVER')
SOURCE_DATABASE = os.getenv('OLAP_DATABASE')
SOURCE_UID = os.getenv('OLAP_UID')
SOURCE_PWD = os.getenv('OLAP_PWD')
SOURCE_TABLE_NAME = 'dbo.tbl_EcomSalesConsolidated'

LOCAL_SERVER = os.getenv('LOCAL_SERVER')
LOCAL_DATABASE = os.getenv('LOCAL_DATABASE')
LOCAL_UID = os.getenv('LOCAL_UID')
LOCAL_PWD = os.getenv('LOCAL_PWD')
LOCAL_DATA_DB = os.getenv('LOCAL_DATABASE', 'local_project_db')
LOCAL_TABLE_NAME = 'dbo.tbl_EcomSalesConsolidated'

def connect_to_mssql(server, database, uid, pwd, autocommit=False):
    try:
        drivers = ['ODBC Driver 18 for SQL Server', 'ODBC Driver 17 for SQL Server']
        connection = None
        for driver in drivers:
            try:
                conn_str = f'DRIVER={{{driver}}};SERVER={server};DATABASE={database};UID={uid};PWD={pwd};TrustServerCertificate=yes;'
                connection = pyodbc.connect(conn_str, autocommit=autocommit)
                print(f"✅ Успешное подключение к базе данных: {database} на {server} с драйвером {driver} (autocommit={autocommit})")
                return connection
            except pyodbc.Error:
                continue

        if not connection:
            raise Exception("❌ Не найден подходящий ODBC драйвер для SQL Server.")
    except Exception as e:
        print(f"❌ Ошибка подключения к базе данных: {e}")
        return None

def sync_normalized(cursor):
    """Инкрементальная синхронизация нормализованной таблицы с mapping-данными"""
    print("Синхронизация tbl_EcomSalesNormalized...")

    # 1. Удаляем записи, чьи source_id обновлялись за последние сутки
    cursor.execute("""
        DELETE n
        FROM dbo.tbl_EcomSalesNormalized n
        WHERE n.source_id IN (
            SELECT id FROM dbo.tbl_EcomSalesConsolidated
            WHERE updated_at >= DATEADD(day, -1, GETDATE())
        )
    """)
    deleted = cursor.rowcount
    print(f"  Удалено: {deleted} строк")

    # 2. Вставляем заново через UNPIVOT
    cursor.execute("""
        INSERT INTO dbo.tbl_EcomSalesNormalized 
            (source_id, [year], [month], brandName, productName, networkName, 
             metric_type, metric_value, updated_at)
        SELECT
            id, [year], [month], brandName, productName, networkName,
            metric_type, metric_value, updated_at
        FROM dbo.tbl_EcomSalesConsolidated
        UNPIVOT (
            metric_value FOR metric_type IN (
                qty, rub, qty_ZC, rub_ZC, qty_PLZ, rub_PLZ,
                qty_AR, rub_AR, qty_OZ, rub_OZ, qty_EA, rub_EA,
                qty_PMP, rub_PMP, qty_OMNI, rub_OMNI,
                qty_NW, rub_NW, NW_wo_ecom, SS_wo_ecom,
                rub_NW_wo_ecom, rub_SS_wo_ecom
            )
        ) AS unpvt
        WHERE metric_value != 0 AND metric_value IS NOT NULL
          AND updated_at >= DATEADD(day, -1, GETDATE())
    """)
    inserted = cursor.rowcount
    print(f"  Вставлено: {inserted} строк")

    # 3. Заполняем mapping-данные (un_rub, segment, channel) для новых строк
    cursor.execute("""
        UPDATE n
        SET n.un_rub  = m.un_rub,
            n.segment = m.segment,
            n.channel = m.channel
        FROM dbo.tbl_EcomSalesNormalized n
        JOIN dbo.tbl_ChannelSegmentMapping m ON n.metric_type = m.name
        WHERE n.un_rub IS NULL
    """)
    updated = cursor.rowcount
    print(f"  Mapping обновлён: {updated} строк")

    cursor.commit()
    print("✅ tbl_EcomSalesNormalized синхронизирована")

def main():
    source_conn = None
    local_conn = None
    source_cursor = None
    local_cursor = None
    local_data_conn = None
    local_data_cursor = None

    try:
        print("Подключаюсь к OLAP базе...")
        source_conn = connect_to_mssql(SOURCE_SERVER, SOURCE_DATABASE, SOURCE_UID, SOURCE_PWD)
        if not source_conn:
            sys.exit("Не удалось подключиться к OLAP базе.")

        print("Подключаюсь к локальной базе...")
        local_conn = connect_to_mssql(LOCAL_SERVER, 'master', LOCAL_UID, LOCAL_PWD)
        if not local_conn:
            sys.exit("Не удалось подключиться к локальной базе.")

        source_cursor = source_conn.cursor()
        local_cursor = local_conn.cursor()

        print(f"Проверяю существование локальной БД {LOCAL_DATA_DB}...")
        local_cursor.execute(f"SELECT name FROM sys.databases WHERE name = ?", LOCAL_DATA_DB)
        if not local_cursor.fetchone():
            print(f"Создаю базу данных {LOCAL_DATA_DB}...")
            temp_conn = connect_to_mssql(LOCAL_SERVER, 'master', LOCAL_UID, LOCAL_PWD, autocommit=True)
            if not temp_conn:
                raise Exception("Не удалось открыть временное соединение для создания БД.")
            temp_cursor = temp_conn.cursor()
            create_db_sql = f"CREATE DATABASE [{LOCAL_DATA_DB}]"
            temp_cursor.execute(create_db_sql)
            temp_cursor.close()
            temp_conn.close()
            print(f"База данных {LOCAL_DATA_DB} создана.")
        else:
            print(f"База данных {LOCAL_DATA_DB} уже существует.")

        print(f"Подключаюсь к локальной БД {LOCAL_DATA_DB}...")
        local_data_conn = connect_to_mssql(LOCAL_SERVER, LOCAL_DATA_DB, LOCAL_UID, LOCAL_PWD)
        if not local_data_conn:
            raise Exception("Не удалось подключиться к новой локальной БД.")
        local_data_cursor = local_data_conn.cursor()

        print(f"Получаю структуру таблицы {SOURCE_TABLE_NAME} из OLAP...")
        ddl_query = """
        SELECT
            c.COLUMN_NAME,
            c.DATA_TYPE,
            CASE WHEN c.IS_NULLABLE = 'NO' THEN 'NOT NULL' ELSE 'NULL' END AS NULLABILITY,
            ISNULL(CONVERT(varchar, c.CHARACTER_MAXIMUM_LENGTH), '') AS MAX_LENGTH,
            ISNULL(CONVERT(varchar, c.NUMERIC_PRECISION), '') AS NUM_PRECISION,
            ISNULL(CONVERT(varchar, c.NUMERIC_SCALE), '') AS NUM_SCALE,
            CASE WHEN COLUMNPROPERTY(OBJECT_ID(c.TABLE_SCHEMA+'.'+c.TABLE_NAME), c.COLUMN_NAME, 'IsIdentity') = 1 THEN 'YES' ELSE 'NO' END AS IS_IDENTITY
        FROM INFORMATION_SCHEMA.COLUMNS c
        WHERE c.TABLE_SCHEMA = PARSENAME(?, 2) AND c.TABLE_NAME = PARSENAME(?, 1)
        ORDER BY c.ORDINAL_POSITION;
        """

        source_cursor.execute(ddl_query, [SOURCE_TABLE_NAME, SOURCE_TABLE_NAME])
        columns_info = source_cursor.fetchall()

        pk_query_fixed = """
        SELECT ku.COLUMN_NAME
        FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS AS tc
        INNER JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE AS ku
            ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME AND tc.TABLE_NAME = ku.TABLE_NAME
        WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY' AND tc.TABLE_SCHEMA = PARSENAME(?, 2) AND tc.TABLE_NAME = PARSENAME(?, 1);
        """
        source_cursor.execute(pk_query_fixed, [SOURCE_TABLE_NAME, SOURCE_TABLE_NAME])
        primary_keys = [row.COLUMN_NAME for row in source_cursor.fetchall()]

        table_name_clean = SOURCE_TABLE_NAME.split('.')[-1]
        create_table_sql = f"IF OBJECT_ID('{LOCAL_TABLE_NAME}', 'U') IS NULL BEGIN\nCREATE TABLE {LOCAL_TABLE_NAME} (\n"
        for col in columns_info:
            col_name = col.COLUMN_NAME
            data_type = col.DATA_TYPE.upper()
            nullable = col.NULLABILITY
            max_len = col.MAX_LENGTH if col.MAX_LENGTH else ''
            prec = col.NUM_PRECISION if col.NUM_PRECISION else ''
            scale = col.NUM_SCALE if col.NUM_SCALE else ''
            is_identity = col.IS_IDENTITY == 'YES'

            type_def = data_type.upper()
            no_param_types = {"INT", "BIGINT", "SMALLINT", "TINYINT", "BIT", "DATE", "TIME", "DATETIME", "DATETIME2", "SMALLDATETIME", "IMAGE", "TEXT", "NTEXT"}

            if type_def in no_param_types:
                pass
            elif max_len != '':
                if max_len == '-1':
                    type_def += "(MAX)"
                else:
                    type_def += f"({max_len})"
            elif prec != '' and scale != '':
                type_def += f"({prec}, {scale})"
            elif prec != '':
                type_def += f"({prec})"

            create_table_sql += f"  [{col_name}] {type_def} {'IDENTITY(1,1)' if is_identity else ''} {nullable},\n"

        if primary_keys:
            pk_cols = ', '.join([f"[{pk}]" for pk in primary_keys])
            create_table_sql += f"  CONSTRAINT [PK_{table_name_clean}] PRIMARY KEY ({pk_cols})\n"
        else:
            create_table_sql = create_table_sql.rstrip(",\n") + "\n"

        create_table_sql += ");\nEND"

        print("Создаю таблицу в локальной БД...")
        local_data_cursor.execute(create_table_sql)
        local_data_conn.commit()
        print(f"Таблица {LOCAL_TABLE_NAME} проверена/создана.")

        print(f"Выбираю данные из {SOURCE_TABLE_NAME}...")
        source_cursor.execute(f"SELECT * FROM {SOURCE_TABLE_NAME};")
        rows = source_cursor.fetchall()
        column_names = [column[0] for column in source_cursor.description]
        print(f"Найдено {len(rows)} строк.")

        print(f"Очищаю локальную таблицу {LOCAL_TABLE_NAME}...")
        local_data_cursor.execute(f"SET IDENTITY_INSERT {LOCAL_TABLE_NAME} ON;")
        local_data_cursor.execute(f"DELETE FROM {LOCAL_TABLE_NAME};")

        placeholders = ', '.join(['?' for _ in column_names])
        insert_query = f"INSERT INTO {LOCAL_TABLE_NAME} ({', '.join([f'[{cn}]' for cn in column_names])}) VALUES ({placeholders});"

        print(f"Вставляю {len(rows)} строк в локальную таблицу {LOCAL_TABLE_NAME}...")
        local_data_cursor.fast_executemany = True
        local_data_cursor.executemany(insert_query, [tuple(row) for row in rows])
        local_data_conn.commit()
        local_data_cursor.execute(f"SET IDENTITY_INSERT {LOCAL_TABLE_NAME} OFF;")
        print("✅ Синхронизация завершена успешно!")

        # Синхронизация нормализованной таблицы
        try:
            sync_normalized(local_data_cursor)
        except Exception as e:
            print(f"⚠️ Ошибка синхронизации нормализованной таблицы: {e}")

    except Exception as e:
        print(f"❌ Ошибка во время синхронизации: {e}")
        try:
            if local_data_conn:
                local_data_conn.rollback()
            if local_conn:
                local_conn.rollback()
        except:
            pass
    finally:
        if source_cursor:
            source_cursor.close()
        if source_conn:
            source_conn.close()
        if local_cursor:
            local_cursor.close()
        if local_conn:
            local_conn.close()
        if local_data_cursor:
            local_data_cursor.close()
        if local_data_conn:
            local_data_conn.close()

if __name__ == "__main__":
    main()