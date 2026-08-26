import unittest
from decimal import Decimal

from create_demo_ecom_sales import (
    METRIC_COLUMNS,
    PLATFORM_CODES,
    PairProfile,
    build_month_metrics,
    combined_level_index,
    fill_price_anchors,
    level_curve,
    month_periods,
    normalized_series,
    price_curve,
    require_demo_target,
    split_units,
    trend_index,
)

YEARS = [2023, 2024, 2025, 2026]


def make_profile(**overrides) -> PairProfile:
    defaults = dict(
        network="Демо-сеть 01 «Аврора»",
        sku="DB01-SKU001",
        brand="Демо-бренд 01 «Альфа»",
        base_units=Decimal("2500"),
        level_index={year: Decimal("1") for year in YEARS},
        nw_ratio=Decimal("0.98"),
        nw_price_factor=Decimal("0.83"),
        offline_price_factor=Decimal("1.005"),
        platforms=("ZC", "AR", "OZ"),
        platform_weights=(Decimal("0.42"), Decimal("0.24"), Decimal("0.15")),
        platform_price_factors={
            "ZC": Decimal("0.98"), "AR": Decimal("1.03"), "OZ": Decimal("0.96"),
        },
        ecom_share_from=Decimal("0.07"),
        ecom_share_to=Decimal("0.19"),
        start_index=0,
        wave_phases=(0.21, 0.63),
    )
    defaults.update(overrides)
    return PairProfile(**defaults)


class SegmentDictionaryTests(unittest.TestCase):
    def test_every_platform_has_both_metric_columns(self):
        for code in PLATFORM_CODES:
            self.assertIn(f"qty_{code}", METRIC_COLUMNS)
            self.assertIn(f"rub_{code}", METRIC_COLUMNS)


class TargetSafetyTests(unittest.TestCase):
    def test_target_must_be_demo(self):
        with self.assertRaises(ValueError):
            require_demo_target("local_project_db")
        require_demo_target("local_project_demo_db")


class SplitUnitsTests(unittest.TestCase):
    def test_split_preserves_total(self):
        for total in (0, 1, 7, 33, 1001):
            parts = split_units(total, (Decimal("0.42"), Decimal("0.24"), Decimal("0.15")))
            self.assertEqual(sum(parts), total)
            self.assertTrue(all(part >= 0 for part in parts))

    def test_split_is_deterministic(self):
        weights = (Decimal("0.42"), Decimal("0.24"), Decimal("0.15"))
        self.assertEqual(split_units(97, weights), split_units(97, weights))


class PriceCurveTests(unittest.TestCase):
    def setUp(self):
        self.anchors = {
            2023: Decimal("2846.73"),
            2024: Decimal("2998.07"),
            2025: Decimal("3199.94"),
            2026: Decimal("3502.37"),
        }

    def test_price_follows_promo_anchor_at_mid_year(self):
        for year, anchor in self.anchors.items():
            middle = (price_curve(self.anchors, year, 6) + price_curve(self.anchors, year, 7)) / 2
            self.assertLess(abs(middle - anchor) / anchor, Decimal("0.005"))

    def test_year_boundary_has_no_jump(self):
        december = price_curve(self.anchors, 2024, 12)
        january = price_curve(self.anchors, 2025, 1)
        self.assertLess(abs(january - december) / december, Decimal("0.01"))
        self.assertGreater(january, december)

    def test_missing_year_is_extrapolated(self):
        filled = fill_price_anchors(
            {"DB01-SKU001": {2025: Decimal("1000")}},
            ["DB01-SKU001"],
            [2023, 2024, 2025, 2026],
            Decimal("500"),
        )
        series = filled["DB01-SKU001"]
        self.assertEqual(sorted(series), [2023, 2024, 2025, 2026])
        self.assertLess(series[2024], series[2025])
        self.assertLess(series[2025], series[2026])


class TrendIndexTests(unittest.TestCase):
    def test_declining_promo_lowers_the_level(self):
        # Сеть теряет объём вдвое, рынок стоит на месте: продажи обязаны пойти
        # вниз, иначе спад из промо не доедет до экрана интернет-продаж.
        market = {2023: Decimal("1000"), 2024: Decimal("1000"),
                  2025: Decimal("1000"), 2026: Decimal("1000")}
        series = {2023: Decimal("100"), 2024: Decimal("90"),
                  2025: Decimal("70"), 2026: Decimal("50")}
        index = trend_index(series, market, YEARS)
        self.assertEqual(index[2023], Decimal("1"))
        self.assertLess(index[2026], index[2025])
        self.assertLess(index[2025], index[2024])
        self.assertLess(index[2026], Decimal("1"))

    def test_growing_promo_raises_the_level(self):
        market = {year: Decimal("1000") for year in YEARS}
        series = {2023: Decimal("50"), 2024: Decimal("70"),
                  2025: Decimal("90"), 2026: Decimal("120")}
        index = trend_index(series, market, YEARS)
        self.assertGreater(index[2026], Decimal("1"))
        self.assertGreater(index[2026], index[2023])

    def test_promo_collapse_is_damped(self):
        # Сеть свернула акции почти полностью — это промо-активность, а не
        # обвал продаж, поэтому уровень ограничен снизу.
        market = {year: Decimal("1000") for year in YEARS}
        series = {2023: Decimal("100"), 2026: Decimal("2")}
        index = trend_index(series, market, YEARS)
        self.assertLess(index[2026], Decimal("1"))
        self.assertGreaterEqual(index[2026], Decimal("0.45"))

    def test_market_growth_is_excluded(self):
        # Ряд растёт ровно как рынок — значит доля не изменилась, уровень ровный.
        market = {2023: Decimal("100"), 2024: Decimal("200"),
                  2025: Decimal("400"), 2026: Decimal("800")}
        index = trend_index(dict(market), market, YEARS)
        for year in YEARS:
            self.assertEqual(index[year], Decimal("1"))

    def test_missing_year_keeps_last_level(self):
        series = normalized_series({2023: Decimal("100"), 2026: Decimal("50")}, YEARS)
        self.assertEqual(series[2024], series[2023])
        self.assertEqual(series[2025], series[2023])
        self.assertLess(series[2026], series[2025])

    def test_level_curve_has_no_jump_between_years(self):
        index = combined_level_index(
            {2023: Decimal("1"), 2024: Decimal("0.9"), 2025: Decimal("0.8"), 2026: Decimal("0.7")},
            {year: Decimal("1") for year in YEARS},
            YEARS,
        )
        december = level_curve(index, 2024, 12)
        january = level_curve(index, 2025, 1)
        self.assertLess(abs(january - december) / december, Decimal("0.02"))


class MonthMetricsTests(unittest.TestCase):
    def setUp(self):
        self.profile = make_profile()
        self.price = Decimal("3200")
        self.level = Decimal("1")

    def metrics(self, index: int, month: int) -> dict[str, Decimal]:
        row = build_month_metrics(self.profile, index, month, self.price, self.level)
        self.assertIsNotNone(row)
        return row

    def test_no_sales_before_start(self):
        profile = make_profile(start_index=5)
        self.assertIsNone(build_month_metrics(profile, 4, 5, self.price, self.level))
        self.assertIsNotNone(build_month_metrics(profile, 5, 6, self.price, self.level))

    def test_units_and_money_are_additive(self):
        row = self.metrics(12, 1)
        platform_units = sum(row.get(f"qty_{code}", Decimal(0)) for code in PLATFORM_CODES)
        platform_rub = sum(row.get(f"rub_{code}", Decimal(0)) for code in PLATFORM_CODES)
        self.assertEqual(row["qty"], row["SS_wo_ecom"] + platform_units)
        self.assertEqual(row["rub"], row["rub_SS_wo_ecom"] + platform_rub)

    def test_without_ecom_never_exceeds_total(self):
        for index in range(0, 43):
            row = self.metrics(index, index % 12 + 1)
            self.assertLess(row["SS_wo_ecom"], row["qty"])
            self.assertLess(row["NW_wo_ecom"], row["qty_NW"])
            self.assertLessEqual(row["rub_SS_wo_ecom"], row["rub"])
            self.assertLessEqual(row["rub_NW_wo_ecom"], row["rub_NW"])

    def test_no_negative_values(self):
        for index in range(0, 43):
            row = self.metrics(index, index % 12 + 1)
            self.assertTrue(all(value >= 0 for value in row.values()))

    def test_online_network_keeps_offline_part_positive(self):
        profile = make_profile(
            ecom_share_from=Decimal("0.9"), ecom_share_to=Decimal("0.98"),
            platforms=("OZ",), platform_weights=(Decimal("0.42"),),
            platform_price_factors={"OZ": Decimal("1.01")},
        )
        for index in range(0, 43):
            row = build_month_metrics(profile, index, index % 12 + 1, self.price, self.level)
            self.assertGreater(row["qty"], row["SS_wo_ecom"])
            self.assertGreater(row["SS_wo_ecom"], Decimal(0))

    def test_series_is_smooth_month_over_month(self):
        # Сезонность, разгон и волна вместе не должны давать ступеньку: ряд
        # одной пары «сеть × SKU» виден в детализации сводной как есть.
        previous = None
        for index in range(0, 43):
            row = self.metrics(index, index % 12 + 1)
            if previous is not None:
                step = abs(row["qty"] - previous) / previous
                self.assertLess(step, Decimal("0.20"))
            previous = row["qty"]

    def test_ramp_softens_the_first_months(self):
        profile = make_profile(start_index=10)
        first = build_month_metrics(profile, 10, 7, self.price, self.level)["qty"]
        settled = build_month_metrics(profile, 14, 7, self.price, self.level)["qty"]
        self.assertLess(first, settled)

    def test_effective_price_stays_near_anchor(self):
        row = self.metrics(30, 7)
        effective = row["rub"] / row["qty"]
        self.assertLess(abs(effective - self.price) / self.price, Decimal("0.05"))

    def test_is_deterministic(self):
        self.assertEqual(self.metrics(7, 8), self.metrics(7, 8))


class PeriodTests(unittest.TestCase):
    def test_month_periods_are_continuous(self):
        periods = month_periods((2023, 1), (2026, 7))
        self.assertEqual(len(periods), 43)
        self.assertEqual(periods[0], (2023, 1))
        self.assertEqual(periods[-1], (2026, 7))
        self.assertEqual(periods[12], (2024, 1))

    def test_reversed_range_is_rejected(self):
        with self.assertRaises(ValueError):
            month_periods((2026, 7), (2023, 1))


if __name__ == "__main__":
    unittest.main()
