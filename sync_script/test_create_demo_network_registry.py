import unittest
from decimal import Decimal

import import_network_facts
from create_demo_promo_db import StableSynthetic
import create_demo_network_registry
from create_demo_network_registry import (
    MONTH_DISTRIBUTIONS,
    build_fact_rows,
    build_network_rows,
    build_plan_rows,
    quarter_of,
    require_demo_target,
    rollup_quarters,
)


SALT = "unit-test-demo-mapping-salt"

GEO_ROWS = [
    {"network_name": f"Демо-сеть {index:02d} «Тест»", "kam": f"КАМ {index % 4:02d}",
     "network_type": "Group Pharm" if index % 7 == 0 else "Chain Pharm",
     "top20_segment": "Демо TOP-20"}
    for index in range(1, 25)
]

SKU_BRANDS = {
    "DB01-SKU001": "Демо-бренд 01 «Альфа»",
    "DB01-SKU002": "Демо-бренд 01 «Альфа»",
    "DB02-SKU001": "Демо-бренд 02 «Бета»",
}


def make_shipments(networks, years=(2025, 2026)):
    """Факт по годам заведомо разный: иначе план года не отличить от плана соседнего."""
    rows = []
    for network in networks:
        for sku in SKU_BRANDS:
            for year in years:
                scale = Decimal("1") + Decimal(year - min(years)) / Decimal("2")
                for month in range(1, 13):
                    rows.append({
                        "networkName": network,
                        "productName": sku,
                        "year": year,
                        "month": month,
                        "units": (Decimal("1000") + Decimal(month * 10)) * scale,
                        "rub": (Decimal("500000") + Decimal(month * 1000)) * scale,
                    })
    return rows


def build_registry(fact_year=2026, plan_years=(2025, 2026, 2027), fact_years=(2025, 2026)):
    """Полный набор строк реестра: карточки, профили, факт и планы."""
    synthetic = StableSynthetic(SALT)
    network_rows = build_network_rows(synthetic, GEO_ROWS)
    network_ids = {row[0]: index for index, row in enumerate(network_rows, start=1)}
    profiles = {
        row[0]: {
            "vat_included": row[8], "vat_rate": row[9],
            "month1_pct": row[4], "month2_pct": row[5], "month3_pct": row[6],
            "default_entry_level": row[10], "default_entry_unit": row[11],
        }
        for row in network_rows
    }
    shipments = make_shipments([row["network_name"] for row in GEO_ROWS], fact_years)
    fact_rows, quarterly = build_fact_rows(synthetic, shipments, network_ids, SKU_BRANDS, {})
    plan_rows = build_plan_rows(
        synthetic, quarterly, network_ids, profiles, {}, fact_year, plan_years,
    )
    return network_ids, profiles, fact_rows, quarterly, plan_rows


class RecordingCursor:
    """Курсор-заглушка: запоминает, что и с какими параметрами исполнено."""

    def __init__(self):
        self.fast_executemany = None
        self.calls = []

    def executemany(self, query, values):
        self.calls.append((query, list(values)))


class TargetSafetyTests(unittest.TestCase):
    def test_target_must_be_demo(self):
        with self.assertRaises(ValueError):
            require_demo_target("local_project_db")
        require_demo_target("local_project_demo_db")


class QuarterTests(unittest.TestCase):
    def test_quarter_of_month(self):
        self.assertEqual([quarter_of(m) for m in (1, 3, 4, 6, 7, 9, 10, 12)],
                         [1, 1, 2, 2, 3, 3, 4, 4])


class NetworkRowTests(unittest.TestCase):
    def setUp(self):
        self.synthetic = StableSynthetic(SALT)
        self.rows = build_network_rows(self.synthetic, GEO_ROWS)

    def test_every_network_keeps_name_and_kam(self):
        self.assertEqual(len(self.rows), len(GEO_ROWS))
        for row, source in zip(self.rows, GEO_ROWS):
            self.assertEqual(row[0], source["network_name"])
            self.assertEqual(row[1], source["kam"])

    def test_month_distribution_sums_to_hundred(self):
        # CK_Networks_month_distribution требует ровно 100.
        for row in self.rows:
            self.assertEqual(row[4] + row[5] + row[6], Decimal("100"))
        for distribution in MONTH_DISTRIBUTIONS:
            self.assertEqual(sum(distribution), Decimal("100"))

    def test_constrained_columns_stay_in_range(self):
        for row in self.rows:
            self.assertIn(row[2], ("regular", "warehouse"))
            self.assertIn(row[3], (0, 1))
            self.assertIn(row[10], ("brand", "sku"))
            self.assertIn(row[11], ("rub", "units"))
            self.assertLess(row[9], Decimal("100"))

    def test_group_pharm_becomes_warehouse(self):
        types = {row[0]: row[2] for row in self.rows}
        for source in GEO_ROWS:
            expected = "warehouse" if source["network_type"] == "Group Pharm" else "regular"
            self.assertEqual(types[source["network_name"]], expected)

    def test_is_deterministic(self):
        self.assertEqual(build_network_rows(StableSynthetic(SALT), GEO_ROWS), self.rows)


class FactRowTests(unittest.TestCase):
    def setUp(self):
        self.synthetic = StableSynthetic(SALT)
        self.network_ids = {"Демо-сеть 01 «Тест»": 1}
        self.rows, self.quarterly = build_fact_rows(
            self.synthetic,
            make_shipments(["Демо-сеть 01 «Тест»", "Сеть вне реестра"]),
            self.network_ids,
            SKU_BRANDS,
            {"Демо-сеть 01 «Тест»": Decimal("12")},
        )

    def test_unknown_network_is_skipped(self):
        self.assertTrue(all(row[0] == 1 for row in self.rows))
        self.assertEqual(len(self.rows), len(SKU_BRANDS) * 24)

    def test_facts_are_never_negative(self):
        # CK_NetworkMonthlyFacts_values запрещает отрицательные суммы.
        for row in self.rows:
            self.assertGreaterEqual(row[5], Decimal("0"))
            self.assertGreaterEqual(row[6], Decimal("0"))
            self.assertGreaterEqual(row[7], Decimal("0"))

    def test_rows_are_sku_level(self):
        # Итог бренда приложение собирает само; строка бренда была бы второй суммой.
        for row in self.rows:
            self.assertIn(row[4], SKU_BRANDS)
            self.assertEqual(row[3], SKU_BRANDS[row[4]])

    def test_quarterly_totals_match_monthly_rows(self):
        monthly = Decimal("0")
        for row in self.rows:
            if row[1] == 2026 and row[3] == "Демо-бренд 01 «Альфа»" and quarter_of(row[2]) == 1:
                monthly += row[5]
        self.assertAlmostEqual(
            float(self.quarterly[(1, "Демо-бренд 01 «Альфа»", 2026, 1)]["rub"]),
            float(monthly), places=2,
        )

    def test_investments_follow_the_percent(self):
        for row in self.rows:
            share = row[7] / row[5] * Decimal("100")
            self.assertGreater(share, Decimal("9"))
            self.assertLess(share, Decimal("15"))


class PlanRowTests(unittest.TestCase):
    def setUp(self):
        self.network_ids, self.profiles, _, self.quarterly, self.rows = build_registry()

    def test_every_plan_year_is_present(self):
        years = {row[1] for row in self.rows}
        self.assertEqual(years, {2025, 2026, 2027})

    def test_plan_is_missed_and_beaten(self):
        # Обе стороны обязаны быть: иначе форма «план / факт» выглядит нарисованной.
        for year in (2025, 2026):
            above = below = 0
            for row in self.rows:
                if row[1] != year or row[3] is None:
                    continue
                fact = self.quarterly[(row[0], row[3], year, row[2])]["rub"]
                if fact > row[4]:
                    above += 1
                else:
                    below += 1
            with self.subTest(year=year):
                self.assertGreater(above, 0)
                self.assertGreater(below, 0)

    def test_year_with_fact_plans_from_its_own_year(self):
        # План 2025-го обязан идти от факта 2025-го, а не от последнего года:
        # иначе закрытый год сравнивался бы с чужим объёмом.
        plans = {(row[0], row[2], row[3]): row[4] for row in self.rows if row[1] == 2025}
        self.assertTrue(plans)
        for (network_id, quarter, brand), plan_rub in plans.items():
            if brand is None:
                continue
            fact = self.quarterly[(network_id, brand, 2025, quarter)]["rub"]
            ratio = float(plan_rub) / float(fact)
            self.assertGreater(ratio, 1 / 1.15)
            self.assertLess(ratio, 1 / 0.87)

    def test_year_without_fact_grows_from_the_last_closed_year(self):
        plans = {(row[0], row[2], row[3]): row[4] for row in self.rows if row[1] == 2027}
        self.assertTrue(plans)
        for (network_id, quarter, brand), plan_rub in plans.items():
            if brand is None:
                continue
            fact = self.quarterly[(network_id, brand, 2026, quarter)]["rub"]
            ratio = float(plan_rub) / float(fact)
            self.assertGreater(ratio, 0.95)
            self.assertLess(ratio, 1.19)

    def test_gross_networks_get_a_total_row(self):
        gross_networks = {row[0] for row in self.rows if row[3] is None}
        self.assertTrue(gross_networks, "нет ни одной строки общего объёма")
        for network_id in gross_networks:
            brand_rows = [r for r in self.rows if r[0] == network_id and r[3] is not None]
            self.assertTrue(all(r[7] == 1 for r in brand_rows))
            for row in self.rows:
                if row[0] == network_id and row[3] is None:
                    # Строка общего объёма сама в пул не входит.
                    self.assertEqual(row[7], 0)

    def test_entry_mode_stays_valid(self):
        for row in self.rows:
            self.assertIn(row[11], ("brand", "sku"))
            self.assertIn(row[12], ("rub", "units"))
            self.assertEqual(row[8] + row[9] + row[10], Decimal("100"))

    def test_plan_rows_are_unique_per_quarter_and_brand(self):
        keys = [(row[0], row[1], row[2], row[3]) for row in self.rows]
        self.assertEqual(len(keys), len(set(keys)), "нарушен UQ_NetworkPlans_row")


class RollupTests(unittest.TestCase):
    def setUp(self):
        _, _, _, self.quarterly, self.plan_rows = build_registry()
        self.cursor = RecordingCursor()
        self.rolled = rollup_quarters(self.cursor, self.quarterly, (2025, 2026, 2027))
        self.params = [row for _, batch in self.cursor.calls for row in batch]

    def test_uses_the_production_rollup_verbatim(self):
        # Свод обязан быть тем же запросом, иначе демо и продакшн разойдутся в том,
        # как квартальный факт собирается из строк SKU.
        self.assertIs(create_demo_network_registry.ROLLUP_SQL, import_network_facts.ROLLUP_SQL)
        self.assertTrue(self.cursor.calls, "свод не выполнен ни разу")
        for query, _ in self.cursor.calls:
            self.assertEqual(query, import_network_facts.ROLLUP_SQL)

    def test_binds_month_range_and_quarter_of_the_same_key(self):
        for network_id, year, month_from, month_to, brand, *merge in self.params:
            quarter = merge[2]
            self.assertEqual((month_from, month_to), ((quarter - 1) * 3 + 1, quarter * 3))
            self.assertEqual(quarter_of(month_from), quarter)
            self.assertEqual(quarter_of(month_to), quarter)
            # Вторая половина параметров адресует ту же строку плана.
            self.assertEqual(merge, [network_id, year, quarter, brand])

    def test_both_closed_years_are_rolled_up(self):
        # Планы ведутся на оба закрытых года, и факт обязан дойти до каждого:
        # год с планом, но без квартального факта — та самая пустая вкладка.
        self.assertEqual({row[1] for row in self.params}, {2025, 2026})
        self.assertEqual(self.rolled, len(self.params))

    def test_year_without_plans_is_skipped(self):
        # Свод адресует только плановые годы: квартал без строки плана MERGE
        # завёл бы сам, и в реестре появился бы лишний год.
        cursor = RecordingCursor()
        rolled = rollup_quarters(cursor, self.quarterly, (2026, 2027))
        params = [row for _, batch in cursor.calls for row in batch]
        self.assertTrue(any(key[2] == 2025 for key in self.quarterly))
        self.assertEqual({row[1] for row in params}, {2026})
        self.assertEqual(rolled, len(params))

    def test_every_brand_plan_of_a_closed_year_gets_a_fact(self):
        # Совпадение множеств означает, что MERGE всегда попадает в существующую
        # строку: ни один план не остаётся без факта и ни одна строка не заводится.
        plan_keys = {
            (row[0], row[1], row[2], row[3])
            for row in self.plan_rows if row[1] in (2025, 2026) and row[3] is not None
        }
        rollup_keys = {(row[5], row[6], row[7], row[8]) for row in self.params}
        self.assertEqual(rollup_keys, plan_keys)

    def test_gross_total_row_is_left_without_fact(self):
        # У строки общего объёма brand_as IS NULL, и свод её не адресует:
        # факт по контракту в целом из отгрузок брендов не складывается.
        self.assertTrue(any(row[3] is None for row in self.plan_rows))
        self.assertTrue(all(row[4] is not None for row in self.params))

    def test_is_deterministic(self):
        cursor = RecordingCursor()
        rollup_quarters(cursor, self.quarterly, (2025, 2026, 2027))
        self.assertEqual(cursor.calls, self.cursor.calls)


if __name__ == "__main__":
    unittest.main()
