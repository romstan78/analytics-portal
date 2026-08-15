import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import pandas as pd

import import_promo


def valid_source():
    return pd.DataFrame({
        'SKU': ['SKU-1'],
        'Название сети': ['Сеть'],
        'Год': ['2026'],
        'Месяц': ['август'],
        'Механика/статья затрат': ['Скидка'],
        'План: Promo уп': ['10,5'],
    })


class PrepareDataframeTests(unittest.TestCase):
    def prepare(self, dataframe):
        with tempfile.NamedTemporaryFile(suffix='.xlsx') as file_handle:
            with patch.object(import_promo.pd, 'read_excel', return_value=dataframe):
                return import_promo.prepare_dataframe(Path(file_handle.name))

    def test_converts_and_validates_source(self):
        dataframe, columns = self.prepare(valid_source())

        self.assertEqual(columns[:5], [
            'network_name', 'year', 'month', 'sku', 'mechanics',
        ])
        self.assertEqual(dataframe.iloc[0]['month'], 8)
        self.assertEqual(dataframe.iloc[0]['plan_promo_units'], 10.5)

    def test_rejects_case_insensitive_duplicate_business_keys(self):
        source = pd.concat([valid_source(), valid_source()], ignore_index=True)
        source.loc[1, 'SKU'] = 'sku-1'

        with self.assertRaisesRegex(ValueError, 'Неоднозначные новые строки'):
            self.prepare(source)

    def test_allows_same_business_key_with_distinct_promo_ids(self):
        source = pd.concat([valid_source(), valid_source()], ignore_index=True)
        source.insert(0, 'ID промо', ['101', '102'])

        dataframe, columns = self.prepare(source)

        self.assertIn('id', columns)
        self.assertEqual(dataframe['id'].tolist(), [101, 102])

    def test_ignores_approval_columns_managed_by_application(self):
        source = valid_source()
        source['Borzenkov A'] = ['согласовано']
        source['Sapunova N.'] = ['отклонено']

        _dataframe, columns = self.prepare(source)

        self.assertNotIn('agreement1', columns)
        self.assertNotIn('agreement2', columns)

    def test_rejects_missing_business_key_column(self):
        source = valid_source().drop(columns=['Механика/статья затрат'])

        with self.assertRaisesRegex(ValueError, 'обязательных колонок'):
            self.prepare(source)

    def test_rejects_invalid_nonempty_number(self):
        source = valid_source()
        source.loc[0, 'Год'] = 'не число'

        with self.assertRaisesRegex(ValueError, 'Некорректные значения'):
            self.prepare(source)

    def test_rejects_fractional_integer_and_non_finite_float(self):
        fractional_year = valid_source()
        fractional_year.loc[0, 'Год'] = '2026.5'
        with self.assertRaisesRegex(ValueError, 'Некорректные значения'):
            self.prepare(fractional_year)

        infinite_units = valid_source()
        infinite_units.loc[0, 'План: Promo уп'] = 'inf'
        with self.assertRaisesRegex(ValueError, 'Некорректные значения'):
            self.prepare(infinite_units)


class MergeSqlTests(unittest.TestCase):
    def test_safe_mode_does_not_delete_missing_rows(self):
        sql = import_promo.build_merge_sql(import_promo.BUSINESS_KEY)

        self.assertNotIn('WHEN NOT MATCHED BY SOURCE', sql)
        self.assertIn('WITH (HOLDLOCK)', sql)

    def test_full_snapshot_explicitly_soft_deletes_missing_rows(self):
        sql = import_promo.build_merge_sql(
            import_promo.BUSINESS_KEY,
            full_snapshot=True,
        )

        self.assertIn('WHEN NOT MATCHED BY SOURCE', sql)
        self.assertIn("'SOFT_DELETE'", sql)

    def test_promo_id_is_used_for_matching_but_not_inserted(self):
        sql = import_promo.build_merge_sql(['id', *import_promo.BUSINESS_KEY])

        self.assertIn('s.[id] IS NOT NULL AND t.[id] = s.[id]', sql)
        insert_clause = sql.split('WHEN NOT MATCHED BY TARGET THEN', 1)[1]
        self.assertNotIn('INSERT ([id]', insert_clause)


class FailingMergeCursor:
    def __init__(self):
        self.result = []
        self.closed = False

    def execute(self, sql):
        if 'FROM sys.columns' in sql:
            self.result = [(column,) for column in import_promo.BUSINESS_KEY]
            self.result += [('deleted_at',), ('updated_at',)]
        elif 'CROSS APPLY' in sql:
            self.result = [(0,)]
        elif f'MERGE {import_promo.TABLE_NAME}' in sql:
            raise RuntimeError('simulated merge failure')
        else:
            self.result = []
        return self

    def executemany(self, _sql, _rows):
        return self

    def fetchall(self):
        return self.result

    def fetchone(self):
        return self.result[0]

    def close(self):
        self.closed = True


class FakeConnection:
    def __init__(self):
        self.test_cursor = FailingMergeCursor()
        self.committed = False
        self.rolled_back = False

    def cursor(self):
        return self.test_cursor

    def commit(self):
        self.committed = True

    def rollback(self):
        self.rolled_back = True


class TransactionTests(unittest.TestCase):
    def test_rolls_back_empty_low_level_import(self):
        connection = FakeConnection()
        dataframe = pd.DataFrame(columns=import_promo.BUSINESS_KEY)

        with self.assertRaisesRegex(ValueError, 'Нет строк для импорта'):
            import_promo.import_dataframe(
                connection,
                dataframe,
                import_promo.BUSINESS_KEY,
            )

        self.assertTrue(connection.rolled_back)
        self.assertFalse(connection.committed)

    def test_rolls_back_entire_import_when_merge_fails(self):
        connection = FakeConnection()
        dataframe = pd.DataFrame([{
            'sku': 'SKU-1',
            'network_name': 'Сеть',
            'year': 2026,
            'month': 8,
            'mechanics': 'Скидка',
        }])

        with self.assertRaisesRegex(RuntimeError, 'simulated merge failure'):
            import_promo.import_dataframe(
                connection,
                dataframe,
                import_promo.BUSINESS_KEY,
            )

        self.assertTrue(connection.rolled_back)
        self.assertFalse(connection.committed)
        self.assertTrue(connection.test_cursor.closed)


if __name__ == '__main__':
    unittest.main()
