import pyodbc

# --- ДАННЫЕ ЛОКАЛЬНОЙ БАЗЫ ---
LOCAL_SERVER = 'localhost' # Потому что Docker запущен локально
LOCAL_DATABASE = 'local_project_db'  # Имя новой БД, куда были скопированы данные
LOCAL_UID = 'sa'           # Стандартный суперпользователь MSSQL
LOCAL_PWD = '$#Pfchfytw_0378' # Пароль, который ты указал в docker-compose.yml
LOCAL_TABLE_NAME = 'dbo.tbl_EcomSalesConsolidated' # Имя таблицы в локальной БД
# ---------------------------------------------

def connect_to_mssql(server, database, uid, pwd):
    """Функция для подключения к MSSQL"""
    try:
        # Попробуй сначала версию 18, затем 17
        drivers = ['ODBC Driver 18 for SQL Server', 'ODBC Driver 17 for SQL Server']
        connection = None
        for driver in drivers:
            try:
                conn_str = f'DRIVER={{{driver}}};SERVER={server};DATABASE={database};UID={uid};PWD={pwd};TrustServerCertificate=yes;'
                connection = pyodbc.connect(conn_str)
                print(f"Успешное подключение к базе данных: {database} на {server} с драйвером {driver}")
                return connection
            except pyodbc.Error:
                continue # Пробуем следующий драйвер

        if not connection:
            raise Exception("Не найден подходящий ODBC драйвер для SQL Server.")
    except Exception as e:
        print(f"Ошибка подключения к базе данных: {e}")
        return None

def main():
    print("Подключаюсь к локальной БД для проверки данных...")
    local_conn = connect_to_mssql(LOCAL_SERVER, LOCAL_DATABASE, LOCAL_UID, LOCAL_PWD)
    if not local_conn:
        print("Не удалось подключиться к локальной БД.")
        return

    cursor = local_conn.cursor()

    try:
        # Проверка количества строк
        print("\n--- Проверка количества строк ---")
        cursor.execute(f"SELECT COUNT(*) AS TotalRows FROM {LOCAL_TABLE_NAME};")
        count_row = cursor.fetchone()
        print(f"Всего строк в таблице {LOCAL_TABLE_NAME}: {count_row.TotalRows}")

        # Проверка первых 5 строк
        print("\n--- Первые 5 строк ---")
        cursor.execute(f"SELECT TOP 5 * FROM {LOCAL_TABLE_NAME};")
        rows = cursor.fetchall()
        columns = [column[0] for column in cursor.description]

        print(f"Столбцы: {', '.join(columns)}")
        for row in rows:
            print(row)

        # Проверка структуры таблицы (простой способ)
        print("\n--- Структура таблицы (информационно) ---")
        cursor.execute(f"""
        SELECT
            c.COLUMN_NAME,
            c.DATA_TYPE,
            CASE WHEN c.IS_NULLABLE = 'NO' THEN 'NOT NULL' ELSE 'NULL' END AS NULLABILITY,
            ISNULL(CONVERT(varchar, c.CHARACTER_MAXIMUM_LENGTH), '') AS MAX_LENGTH,
            ISNULL(CONVERT(varchar, c.NUMERIC_PRECISION), '') AS NUM_PRECISION,
            ISNULL(CONVERT(varchar, c.NUMERIC_SCALE), '') AS NUM_SCALE
        FROM INFORMATION_SCHEMA.COLUMNS c
        WHERE c.TABLE_NAME = PARSENAME(?, 1) AND c.TABLE_SCHEMA = PARSENAME(?, 2)
        ORDER BY c.ORDINAL_POSITION;
        """, [LOCAL_TABLE_NAME, LOCAL_TABLE_NAME])

        struct_rows = cursor.fetchall()
        print("Column Name\tData Type\tNullability\tMax Length/Precision/Scale")
        print("-" * 80)
        for r in struct_rows:
             print(f"{r.COLUMN_NAME}\t{r.DATA_TYPE}\t{r.NULLABILITY}\t{r.MAX_LENGTH}/{r.NUM_PRECISION}/{r.NUM_SCALE}")


    except Exception as e:
        print(f"Ошибка при выполнении запроса: {e}")
    finally:
        cursor.close()
        local_conn.close()

if __name__ == "__main__":
    main()
