import unittest
from decimal import Decimal

from create_demo_promo_db import StableSynthetic
from create_demo_network_registry import (
    MONTH_DISTRIBUTIONS,
    build_fact_rows,
    build_network_rows,
    build_plan_rows,
    quarter_of,
    require_demo_target,
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
    rows = []
    for network in networks:
        for sku in SKU_BRANDS:
            for year in years:
                for month in range(1, 13):
                    rows.append({
                        "networkName": network,
                        "productName": sku,
                        "year": year,
                        "month": month,
                        "units": Decimal("1000") + Decimal(month * 10),
                        "rub": Decimal("500000") + Decimal(month * 1000),
                    })
    return rows


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
        self.synthetic = StableSynthetic(SALT)
        networks = [row["network_name"] for row in GEO_ROWS]
        network_rows = build_network_rows(self.synthetic, networks and GEO_ROWS)
        self.network_ids = {row[0]: index for index, row in enumerate(network_rows, start=1)}
        self.profiles = {
            row[0]: {
                "vat_included": row[8], "vat_rate": row[9],
                "month1_pct": row[4], "month2_pct": row[5], "month3_pct": row[6],
                "default_entry_level": row[10], "default_entry_unit": row[11],
            }
            for row in network_rows
        }
        _, self.quarterly = build_fact_rows(
            self.synthetic, make_shipments(networks), self.network_ids, SKU_BRANDS, {},
        )
        self.rows = build_plan_rows(
            self.synthetic, self.quarterly, self.network_ids, self.profiles, {},
            2026, (2026, 2027),
        )

    def test_both_plan_years_are_present(self):
        years = {row[1] for row in self.rows}
        self.assertEqual(years, {2026, 2027})

    def test_plan_is_missed_and_beaten(self):
        # Обе стороны обязаны быть: иначе форма «план / факт» выглядит нарисованной.
        above = below = 0
        for row in self.rows:
            if row[1] != 2026 or row[3] is None:
                continue
            fact = self.quarterly[(row[0], row[3], 2026, row[2])]["rub"]
            if fact > row[4]:
                above += 1
            else:
                below += 1
        self.assertGreater(above, 0)
        self.assertGreater(below, 0)

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


if __name__ == "__main__":
    unittest.main()
