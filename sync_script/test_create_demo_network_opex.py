import unittest
from collections import defaultdict
from decimal import Decimal

from create_demo_promo_db import StableSynthetic
from create_demo_network_opex import (
    ARTICLES,
    MIN_ARTICLES,
    OPEX_PCT_BOUNDS,
    UPDATED_BY,
    article_weights,
    build_opex_rows,
    net_rub,
    network_opex_pct,
    require_demo_target,
    split_kopecks,
    summarize_networks,
)


SALT = "unit-test-demo-mapping-salt"

NETWORKS = [
    {"id": 1, "name": "Демо-сеть 01 «Тест»", "kam": "КАМ 01", "is_active": True,
     "vat_included": True, "vat_rate": Decimal("20.00")},
    {"id": 2, "name": "Демо-сеть 02 «Тест»", "kam": "КАМ 02", "is_active": True,
     "vat_included": False, "vat_rate": Decimal("20.00")},
]

TURNOVER = {
    (1, 2025): {"Бренд А": Decimal("6000000.00"), "Бренд Б": Decimal("4000000.00")},
    (1, 2026): {"Бренд А": Decimal("9000000.00"), "Бренд Б": Decimal("3000000.00")},
    (2, 2025): {"Бренд А": Decimal("2500000.00")},
    # У сети 2 в 2026 году плана нет — бюджета быть не должно.
}

# У сети 1 второй квартал 2025 года заведён без НДС — вопреки карточке.
PERIOD_VAT = {
    (1, 2025, 2): (False, Decimal("20.00")),
}


def cells_by_key(cells):
    grouped = defaultdict(list)
    for cell in cells:
        grouped[(cell["network_id"], cell["year"], cell["brand_as"], cell["article"])].append(cell)
    return grouped


class SplitKopecksTest(unittest.TestCase):
    """Те же случаи, что и в TestSplitOpexKopecks на сервере: правило одно."""

    def test_matches_server_cases(self):
        cases = {
            "300": ("100", "100", "100"),
            "100.01": ("33.33", "33.33", "33.35"),
            "100": ("33.33", "33.33", "33.34"),
            "-100.01": ("-33.33", "-33.33", "-33.35"),
            "0": ("0", "0", "0"),
            "0.01": ("0", "0", "0.01"),
        }
        for amount, expected in cases.items():
            with self.subTest(amount=amount):
                got = split_kopecks(Decimal(amount))
                self.assertEqual(got, tuple(Decimal(value) for value in expected))

    def test_keeps_sum_and_kopecks(self):
        for kopecks in range(-250, 251):
            amount = Decimal(kopecks) / 100
            got = split_kopecks(amount)
            self.assertEqual(sum(got), amount)
            for value in got:
                self.assertEqual(value, value.quantize(Decimal("0.01")))


class NetRubTest(unittest.TestCase):
    def test_vat_removed_when_included(self):
        self.assertEqual(net_rub(Decimal("120"), True, Decimal("20")), Decimal("100.00"))

    def test_unchanged_without_vat(self):
        self.assertEqual(net_rub(Decimal("99.99"), False, Decimal("20")), Decimal("99.99"))
        self.assertEqual(net_rub(Decimal("99.99"), True, Decimal("0")), Decimal("99.99"))


class RuleTest(unittest.TestCase):
    def setUp(self):
        self.synthetic = StableSynthetic(SALT)

    def test_pct_within_bounds_and_stable(self):
        low, high = (Decimal(bound) for bound in OPEX_PCT_BOUNDS)
        for index in range(1, 61):
            name = f"Демо-сеть {index:02d}"
            pct = network_opex_pct(self.synthetic, name)
            self.assertGreaterEqual(pct, low)
            self.assertLessEqual(pct, high)
            self.assertEqual(pct, network_opex_pct(StableSynthetic(SALT), name))

    def test_pct_varies_between_networks(self):
        values = {network_opex_pct(self.synthetic, f"Демо-сеть {index:02d}") for index in range(1, 61)}
        self.assertGreater(len(values), 10)

    def test_article_weights(self):
        codes = {code for code, _ in ARTICLES}
        absent_somewhere = False
        for index in range(1, 61):
            weights = article_weights(self.synthetic, f"Демо-сеть {index:02d}")
            self.assertTrue(set(weights) <= codes)
            self.assertGreaterEqual(len(weights), MIN_ARTICLES)
            self.assertAlmostEqual(float(sum(weights.values())), 1.0, places=9)
            absent_somewhere = absent_somewhere or len(weights) < len(ARTICLES)
        self.assertTrue(absent_somewhere, "ни у одной сети не отсутствует ни одна статья")

    def test_require_demo_target(self):
        require_demo_target("local_project_demo_db")
        with self.assertRaises(ValueError):
            require_demo_target("local_project_db")


class BuildRowsTest(unittest.TestCase):
    def setUp(self):
        self.synthetic = StableSynthetic(SALT)
        self.rows, self.cells = build_opex_rows(
            self.synthetic, NETWORKS, TURNOVER, PERIOD_VAT, (2025, 2026),
        )

    def test_year_without_plan_gets_no_budget(self):
        self.assertFalse(any(row[0] == 2 and row[1] == 2026 for row in self.rows))
        self.assertTrue(any(row[0] == 2 and row[1] == 2025 for row in self.rows))

    def test_brands_only_from_plan(self):
        for row in self.rows:
            self.assertIn(row[3], TURNOVER[(row[0], row[1])])
            self.assertEqual(row[7], UPDATED_BY)

    def test_each_cell_has_three_months_per_quarter_and_equal_quarters(self):
        for key, quarter_cells in cells_by_key(self.cells).items():
            self.assertEqual([cell["quarter"] for cell in quarter_cells], [1, 2, 3, 4], key)
            amounts = {cell["amount_rub"] for cell in quarter_cells}
            self.assertEqual(len(amounts), 1, f"кварталы {key} неравны: {amounts}")
            amount = amounts.pop()
            self.assertEqual(amount, amount.quantize(Decimal("1")), "квартал не округлён до рубля")
            self.assertGreater(amount, 0)

        months = defaultdict(list)
        for row in self.rows:
            months[(row[0], row[1], row[3], row[4], (row[2] - 1) // 3 + 1)].append(row)
        for key, month_rows in months.items():
            self.assertEqual(len(month_rows), 3, key)
            gross = sum(row[5] for row in month_rows)
            net = sum(row[6] for row in month_rows)
            cell = next(
                c for c in self.cells
                if (c["network_id"], c["year"], c["brand_as"], c["article"], c["quarter"]) == key
            )
            self.assertEqual(gross, cell["amount_rub"])
            self.assertEqual(net, cell["amount_rub_net"])

    def test_total_is_pct_of_turnover(self):
        for (network_id, year), brands in TURNOVER.items():
            network = next(n for n in NETWORKS if n["id"] == network_id)
            total = sum(
                cell["amount_rub"] for cell in self.cells
                if cell["network_id"] == network_id and cell["year"] == year
            )
            expected = sum(brands.values()) * network_opex_pct(self.synthetic, network["name"]) / 100
            # Каждая ячейка округлена до рубля в каждом квартале: расхождение
            # не больше 2 рублей на ячейку (4 квартала × 0.5 руб).
            cell_count = sum(
                1 for cell in self.cells
                if cell["network_id"] == network_id and cell["year"] == year and cell["quarter"] == 1
            )
            self.assertLessEqual(abs(total - expected), Decimal(2) * cell_count, (network_id, year))

    def test_vat_follows_period_not_card(self):
        for cell in self.cells:
            gross, net = cell["amount_rub"], cell["amount_rub_net"]
            if cell["network_id"] == 2 or (cell["network_id"], cell["year"], cell["quarter"]) in PERIOD_VAT:
                self.assertEqual(gross, net, cell)
            else:
                self.assertEqual(net, (gross / Decimal("1.2")).quantize(Decimal("0.01")), cell)

    def test_years_differ_and_pct_is_shared(self):
        by_year = {}
        for cell in self.cells:
            if cell["network_id"] == 1 and cell["brand_as"] == "Бренд А" and cell["quarter"] == 1:
                by_year.setdefault(cell["year"], {})[cell["article"]] = cell["amount_rub"]
        self.assertEqual(set(by_year), {2025, 2026})
        self.assertNotEqual(by_year[2025], by_year[2026])
        pcts = {cell["pct"] for cell in self.cells if cell["network_id"] == 1}
        self.assertEqual(len(pcts), 1)

    def test_summary_sums(self):
        summary = summarize_networks(self.cells)
        self.assertEqual([(row["network_id"], row["year"]) for row in summary], [(1, 2025), (1, 2026), (2, 2025)])
        for row in summary:
            quarters = sum(row[f"q{quarter}_rub"] for quarter in (1, 2, 3, 4))
            articles = sum(row[f"{code}_rub"] for code, _ in ARTICLES)
            self.assertEqual(row["budget_rub"], quarters)
            self.assertEqual(row["budget_rub"], articles)
            self.assertEqual(row["q1_rub"], row["q4_rub"])
            self.assertEqual(row["brands"], len(TURNOVER[(row["network_id"], row["year"])]))
            self.assertLessEqual(abs(row["effective_pct"] - row["pct"]), Decimal("0.05"))


if __name__ == "__main__":
    unittest.main()
