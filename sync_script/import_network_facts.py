"""Загрузка фактических объёмов и инвестиций в планы реестра сетей.

Факт в реестре не вводится руками: он приходит выгрузкой отгрузок и ложится
в колонки fact_rub и fact_investments_rub таблицы dbo.tbl_NetworkPlans.

Ожидаемые колонки файла (Excel или CSV), регистр и лишние пробелы не важны:

    Сеть | Бренд | Год | Квартал | Факт, руб | Факт инвестиций, руб

«Факт инвестиций, руб» необязателен. Сеть сопоставляется с dbo.tbl_Networks.name,
бренд — со строкой плана по brand_as. Строки, которых в плане ещё нет, создаются:
факт может прийти по бренду, который не планировали.

    # Проверить файл, ничего не записывая
    python sync_script/import_network_facts.py /path/to/facts.xlsx --dry-run

    # Загрузить
    python sync_script/import_network_facts.py /path/to/facts.xlsx
"""

import argparse
import difflib
import os
import sys
from pathlib import Path

import pandas as pd
import pyodbc
from dotenv import load_dotenv

load_dotenv()

LOCAL_SERVER = os.getenv('LOCAL_SERVER')
LOCAL_DATABASE = os.getenv('LOCAL_DATABASE', 'local_project_db')
LOCAL_UID = os.getenv('LOCAL_UID')
LOCAL_PWD = os.getenv('LOCAL_PWD')

# Заголовок файла → поле. Ключи приводятся к нижнему регистру без пробелов,
# поэтому «Факт, руб» и «факт руб» распознаются одинаково.
COLUMN_ALIASES = {
    'сеть': 'network_name',
    'названиесети': 'network_name',
    'бренд': 'brand_as',
    'брендляас': 'brand_as',
    'бренддляас': 'brand_as',
    'год': 'year',
    'квартал': 'quarter',
    'фактруб': 'fact_rub',
    'факт': 'fact_rub',
    'фактобъёмруб': 'fact_rub',
    'фактобъемруб': 'fact_rub',
    'фактинвестицийруб': 'fact_investments_rub',
    'фактинвестиций': 'fact_investments_rub',
    'фактинвестиции': 'fact_investments_rub',
}

REQUIRED = ['network_name', 'brand_as', 'year', 'quarter']
VALUE_COLUMNS = ['fact_rub', 'fact_investments_rub']


class ImportError_(Exception):
    """Ошибка подготовки данных: файл не годится для загрузки."""


def normalize_header(name):
    """«Факт, руб » → «фактруб»: сравниваем заголовки без регистра и пунктуации."""
    return ''.join(ch for ch in str(name).lower() if ch.isalnum())


def parse_number(value):
    """Число из ячейки: пустое — None, «1 234,50» — 1234.5."""
    if value is None or (isinstance(value, float) and pd.isna(value)):
        return None
    text = str(value).strip().replace('\xa0', '').replace(' ', '').replace(',', '.')
    if text == '' or text.lower() in {'nan', 'none', '-', '—'}:
        return None
    try:
        return float(text)
    except ValueError as exc:
        raise ImportError_(f'не число: {value!r}') from exc


def parse_int(value, field):
    number = parse_number(value)
    if number is None:
        raise ImportError_(f'{field}: пустое значение')
    if number != int(number):
        raise ImportError_(f'{field}: ожидалось целое, получено {value!r}')
    return int(number)


def clean_text(value):
    if value is None or (isinstance(value, float) and pd.isna(value)):
        return None
    text = str(value).strip()
    return text or None


def read_source(path):
    """Читает Excel или CSV в DataFrame строк."""
    if path.suffix.lower() in {'.csv', '.tsv'}:
        separator = '\t' if path.suffix.lower() == '.tsv' else None
        return pd.read_csv(path, dtype=str, sep=separator, engine='python')
    return pd.read_excel(path, dtype=str)


def prepare_dataframe(source):
    """Приводит выгрузку к строкам загрузки и проверяет её целиком.

    Возвращает список словарей. Бросает ImportError_ с перечнем всех проблем,
    чтобы файл не приходилось чинить построчно за несколько заходов.
    """
    rename = {}
    for column in source.columns:
        field = COLUMN_ALIASES.get(normalize_header(column))
        if field and field not in rename.values():
            rename[column] = field

    frame = source.rename(columns=rename)
    missing = [field for field in REQUIRED if field not in frame.columns]
    if missing:
        raise ImportError_('в файле нет обязательных колонок: ' + ', '.join(missing))
    if not any(field in frame.columns for field in VALUE_COLUMNS):
        raise ImportError_('в файле нет ни «Факт, руб», ни «Факт инвестиций, руб»')

    rows = []
    problems = []
    seen = {}

    for position, record in frame.iterrows():
        line = int(position) + 2  # +1 на заголовок, +1 на нумерацию с единицы
        try:
            network = clean_text(record.get('network_name'))
            brand = clean_text(record.get('brand_as'))
            if not network:
                raise ImportError_('пустая сеть')
            if not brand:
                raise ImportError_('пустой бренд')

            year = parse_int(record.get('year'), 'год')
            quarter = parse_int(record.get('quarter'), 'квартал')
            if year < 2000 or year > 2100:
                raise ImportError_(f'год вне диапазона 2000–2100: {year}')
            if quarter < 1 or quarter > 4:
                raise ImportError_(f'квартал вне диапазона 1–4: {quarter}')

            fact = parse_number(record.get('fact_rub')) if 'fact_rub' in frame.columns else None
            investments = (
                parse_number(record.get('fact_investments_rub'))
                if 'fact_investments_rub' in frame.columns else None
            )
            if fact is None and investments is None:
                continue  # пустая строка выгрузки — просто пропускаем
            for label, number in (('факт', fact), ('факт инвестиций', investments)):
                if number is not None and number < 0:
                    raise ImportError_(f'{label} отрицательный: {number}')

            key = (network.lower(), brand.lower(), year, quarter)
            if key in seen:
                raise ImportError_(f'дубль строки, уже встречалась в строке {seen[key]}')
            seen[key] = line

            rows.append({
                'network_name': network,
                'brand_as': brand,
                'year': year,
                'quarter': quarter,
                'fact_rub': fact,
                'fact_investments_rub': investments,
            })
        except ImportError_ as exc:
            problems.append(f'строка {line}: {exc}')

    if problems:
        raise ImportError_('файл не прошёл проверку:\n  ' + '\n  '.join(problems))
    if not rows:
        raise ImportError_('в файле нет строк со значениями факта')
    return rows


def connect_to_db():
    drivers = ['ODBC Driver 18 for SQL Server', 'ODBC Driver 17 for SQL Server']
    for driver in drivers:
        try:
            return pyodbc.connect(
                f'DRIVER={{{driver}}};SERVER={LOCAL_SERVER};DATABASE={LOCAL_DATABASE};'
                f'UID={LOCAL_UID};PWD={LOCAL_PWD};TrustServerCertificate=yes;'
            )
        except pyodbc.Error:
            continue
    raise ImportError_('не найден ODBC-драйвер 17 или 18 для SQL Server')


def load_networks(cursor):
    """Реестр сетей: (название в нижнем регистре → id, список названий как есть)."""
    cursor.execute('SELECT id, name FROM dbo.tbl_Networks')
    rows = [(network_id, (name or '').strip()) for network_id, name in cursor.fetchall()]
    return {name.lower(): network_id for network_id, name in rows}, [name for _, name in rows]


def resolve_networks(rows, network_ids):
    """Проставляет network_id. Возвращает (готовые строки, ненайденные сети)."""
    resolved = []
    unknown = set()
    for row in rows:
        network_id = network_ids.get(row['network_name'].strip().lower())
        if network_id is None:
            unknown.add(row['network_name'])
            continue
        resolved.append({**row, 'network_id': network_id})
    return resolved, sorted(unknown)


def suggest_similar(name, known_names, limit=2):
    """Похожие названия из реестра: опечатку и лишний суффикс видно сразу,
    а не после ручного сравнения двух списков."""
    lookup = {known.lower(): known for known in known_names}
    matches = difflib.get_close_matches(name.strip().lower(), list(lookup), n=limit, cutoff=0.6)
    return [lookup[match] for match in matches]


def describe_unknown(unknown, known_names):
    """Строки отчёта по ненайденным сетям — с подсказкой, если есть похожие."""
    lines = []
    for name in unknown:
        similar = suggest_similar(name, known_names)
        if similar:
            lines.append(f'  «{name}» — похоже на: {", ".join(similar)}')
        else:
            lines.append(f'  «{name}» — похожих в реестре нет')
    return lines


# Факт кладётся на существующую строку плана либо заводит новую:
# бренд мог отгрузиться без плана, и такой факт терять нельзя.
#
# Типы параметров заданы явно: без CAST бренд сравнивается как VARCHAR,
# и кириллица не находит существующую строку плана.
MERGE_SQL = """
MERGE dbo.tbl_NetworkPlans AS t
USING (VALUES (
    CAST(? AS INT), CAST(? AS INT), CAST(? AS INT),
    CAST(? AS NVARCHAR(255)), CAST(? AS DECIMAL(18,2)), CAST(? AS DECIMAL(18,2))
)) AS s (network_id, [year], [quarter], brand_as, fact_rub, fact_investments_rub)
    ON  t.network_id = s.network_id
    AND t.[year] = s.[year]
    AND t.[quarter] = s.[quarter]
    AND t.brand_as = s.brand_as
WHEN MATCHED THEN UPDATE SET
    fact_rub = COALESCE(s.fact_rub, t.fact_rub),
    fact_investments_rub = COALESCE(s.fact_investments_rub, t.fact_investments_rub),
    updated_at = GETDATE()
WHEN NOT MATCHED THEN
    INSERT (network_id, [year], [quarter], brand_as, in_gross, fact_rub, fact_investments_rub, updated_by)
    VALUES (s.network_id, s.[year], s.[quarter], s.brand_as, 0, s.fact_rub, s.fact_investments_rub, 'import_network_facts');
"""


def import_rows(connection, rows):
    """Пишет факт одной транзакцией. Возвращает число обработанных строк."""
    cursor = connection.cursor()
    try:
        # fast_executemany здесь не включаем: он биндит строки как ANSI,
        # и названия брендов кириллицей уходят в базу знаками вопроса.
        # Объёмы загрузки — тысячи строк, скорость обычного executemany достаточна.
        cursor.executemany(MERGE_SQL, [
            (row['network_id'], row['year'], row['quarter'], row['brand_as'],
             row['fact_rub'], row['fact_investments_rub'])
            for row in rows
        ])
        connection.commit()
        return len(rows)
    except Exception:
        connection.rollback()
        raise
    finally:
        cursor.close()


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description='Загрузка факта объёмов и инвестиций в планы реестра сетей.',
    )
    parser.add_argument('source_file', type=Path, help='Excel или CSV с фактом')
    parser.add_argument(
        '--dry-run', action='store_true',
        help='только проверить файл и сопоставление сетей, ничего не записывая',
    )
    parser.add_argument(
        '--allow-unknown-networks', action='store_true',
        help='пропустить строки сетей, которых нет в реестре, вместо остановки',
    )
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    if not args.source_file.exists():
        print(f'Файл не найден: {args.source_file}')
        return 1

    try:
        rows = prepare_dataframe(read_source(args.source_file))
    except ImportError_ as exc:
        print(f'Проверка не пройдена: {exc}')
        return 1

    print(f'Строк со значениями факта: {len(rows)}')

    try:
        connection = connect_to_db()
    except ImportError_ as exc:
        print(f'Нет подключения к базе: {exc}')
        return 1

    try:
        cursor = connection.cursor()
        network_ids, known_names = load_networks(cursor)
        cursor.close()
        resolved, unknown = resolve_networks(rows, network_ids)

        if unknown:
            print(f'Не найдено сетей в реестре: {len(unknown)}')
            print('\n'.join(describe_unknown(unknown, known_names)))
            if not args.allow_unknown_networks:
                print('Переименуйте их в файле, заведите в реестре '
                      'или запустите с --allow-unknown-networks.')
                return 1
            print(f'Пропускаю {len(rows) - len(resolved)} строк по этим сетям.')

        if not resolved:
            print('Сопоставить не удалось ни одной строки.')
            return 1

        if args.dry_run:
            print(f'Проверка пройдена. К загрузке готово {len(resolved)} строк, запись не выполнялась.')
            return 0

        written = import_rows(connection, resolved)
        print(f'Загружено строк: {written}')
        return 0
    finally:
        connection.close()


if __name__ == '__main__':
    sys.exit(main())
