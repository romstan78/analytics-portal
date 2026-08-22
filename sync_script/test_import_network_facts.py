import unittest

import pandas as pd

import import_network_facts as facts


def valid_source():
    return pd.DataFrame({
        'Сеть': ['Магнит', 'Магнит'],
        'Бренд': ['Альфа', 'Бета'],
        'Год': ['2026', '2026'],
        'Квартал': ['1', '1'],
        'Факт, руб': ['5 120 000', '1 755 000,50'],
        'Факт инвестиций, руб': ['512000', ''],
    })


class PrepareDataframeTests(unittest.TestCase):
    def test_reads_values_and_normalizes_numbers(self):
        rows = facts.prepare_dataframe(valid_source())

        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]['network_name'], 'Магнит')
        self.assertEqual(rows[0]['year'], 2026)
        self.assertEqual(rows[0]['quarter'], 1)
        self.assertEqual(rows[0]['fact_rub'], 5120000)
        self.assertEqual(rows[0]['fact_investments_rub'], 512000)
        # Пустой факт инвестиций — это отсутствие значения, а не ноль.
        self.assertEqual(rows[1]['fact_rub'], 1755000.5)
        self.assertIsNone(rows[1]['fact_investments_rub'])

    def test_headers_are_matched_case_and_punctuation_insensitively(self):
        source = valid_source().rename(columns={'Факт, руб': 'ФАКТ РУБ', 'Сеть': ' сеть '})
        rows = facts.prepare_dataframe(source)
        self.assertEqual(rows[0]['fact_rub'], 5120000)

    def test_accepts_file_without_investments_column(self):
        source = valid_source().drop(columns=['Факт инвестиций, руб'])
        rows = facts.prepare_dataframe(source)
        self.assertIsNone(rows[0]['fact_investments_rub'])

    def test_rejects_missing_required_column(self):
        source = valid_source().drop(columns=['Квартал'])
        with self.assertRaises(facts.ImportError_) as ctx:
            facts.prepare_dataframe(source)
        self.assertIn('quarter', str(ctx.exception))

    def test_rejects_file_without_any_value_column(self):
        source = valid_source().drop(columns=['Факт, руб', 'Факт инвестиций, руб'])
        with self.assertRaises(facts.ImportError_):
            facts.prepare_dataframe(source)

    def test_rejects_duplicate_rows(self):
        source = pd.concat([valid_source(), valid_source().head(1)], ignore_index=True)
        with self.assertRaises(facts.ImportError_) as ctx:
            facts.prepare_dataframe(source)
        self.assertIn('дубль', str(ctx.exception))

    def test_rejects_bad_quarter_and_negative_values(self):
        source = valid_source()
        source.loc[0, 'Квартал'] = '5'
        source.loc[1, 'Факт, руб'] = '-100'
        with self.assertRaises(facts.ImportError_) as ctx:
            facts.prepare_dataframe(source)
        message = str(ctx.exception)
        self.assertIn('квартал вне диапазона', message)
        self.assertIn('отрицательный', message)

    def test_rejects_non_numeric_value(self):
        source = valid_source()
        source.loc[0, 'Факт, руб'] = 'нет данных'
        with self.assertRaises(facts.ImportError_) as ctx:
            facts.prepare_dataframe(source)
        self.assertIn('не число', str(ctx.exception))

    def test_skips_rows_without_any_value(self):
        source = valid_source()
        source.loc[1, 'Факт, руб'] = ''
        source.loc[1, 'Факт инвестиций, руб'] = ''
        rows = facts.prepare_dataframe(source)
        self.assertEqual(len(rows), 1)

    def test_rejects_file_where_every_row_is_empty(self):
        source = valid_source()
        source['Факт, руб'] = ''
        source['Факт инвестиций, руб'] = ''
        with self.assertRaises(facts.ImportError_):
            facts.prepare_dataframe(source)


class ResolveNetworksTests(unittest.TestCase):
    def test_matches_network_ignoring_case_and_spaces(self):
        rows = facts.prepare_dataframe(valid_source())
        resolved, unknown = facts.resolve_networks(rows, {'магнит': 7})

        self.assertEqual(unknown, [])
        self.assertTrue(all(row['network_id'] == 7 for row in resolved))

    def test_reports_networks_missing_from_registry(self):
        rows = facts.prepare_dataframe(valid_source())
        resolved, unknown = facts.resolve_networks(rows, {})

        self.assertEqual(resolved, [])
        self.assertEqual(unknown, ['Магнит'])


class SuggestSimilarTests(unittest.TestCase):
    KNOWN = ['Магнит', 'Пятёрочка', 'Лента']

    def test_suggests_registry_name_for_typo(self):
        self.assertEqual(facts.suggest_similar('Магнмт', self.KNOWN), ['Магнит'])

    def test_suggests_registry_name_for_extra_suffix(self):
        self.assertIn('Магнит', facts.suggest_similar('магнит ММ', self.KNOWN))

    def test_returns_nothing_for_unrelated_name(self):
        self.assertEqual(facts.suggest_similar('Ашан', self.KNOWN), [])

    def test_report_marks_names_without_any_match(self):
        lines = facts.describe_unknown(['Магнмт', 'Ашан'], self.KNOWN)

        self.assertIn('Магнит', lines[0])
        self.assertIn('похожих в реестре нет', lines[1])


class MergeSqlTests(unittest.TestCase):
    def test_merge_keeps_existing_value_when_column_absent(self):
        # COALESCE: пустая колонка выгрузки не затирает уже загруженное значение.
        self.assertIn('COALESCE(s.fact_rub, t.fact_rub)', facts.MERGE_SQL)
        self.assertIn('COALESCE(s.fact_investments_rub, t.fact_investments_rub)', facts.MERGE_SQL)

    def test_merge_creates_row_for_unplanned_brand(self):
        self.assertIn('WHEN NOT MATCHED THEN', facts.MERGE_SQL)
        # Новая строка не попадает в валовый объём сама по себе.
        self.assertIn('in_gross', facts.MERGE_SQL)


if __name__ == '__main__':
    unittest.main()
