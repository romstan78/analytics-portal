import unittest
from datetime import date
from decimal import Decimal

from create_demo_promo_db import (
    StableSynthetic,
    build_entity_maps,
    canonical_kam,
    require_target_safety,
    transform_promo_row,
)


class DemoPromoTransformTests(unittest.TestCase):
    def setUp(self):
        self.synthetic = StableSynthetic("unit-test-demo-mapping-salt")

    def test_kam_order_is_canonical(self):
        self.assertEqual(
            canonical_kam("Марина Алексеева"),
            canonical_kam("Алексеева Марина"),
        )

    def test_target_must_be_demo_and_different(self):
        with self.assertRaises(ValueError):
            require_target_safety("localhost,1433", "prod", "localhost,1433", "prod")
        with self.assertRaises(ValueError):
            require_target_safety("localhost,1433", "prod", "localhost,1434", "copy")
        require_target_safety(
            "localhost,1433", "local_project_db",
            "localhost,1434", "local_project_demo_db",
        )

    def test_transform_preserves_year_and_synthesizes_sensitive_text(self):
        source = {
            "id": 77,
            "network_name": "Исходная сеть",
            "kam": "Марина Алексеева",
            "brand": "Исходный бренд",
            "brand_as": "Исходный бренд",
            "sku": "Исходный SKU",
            "year": 2025,
            "month": 5,
            "quarter": 2,
            "mechanics": "Скидка",
            "baseline_units": Decimal("100"),
            "plan_promo_units": Decimal("150"),
            "plan_investments_rub": Decimal("10000"),
            "contract_price": Decimal("500"),
            "gm": Decimal("0.40"),
            "promo_pharmacies": 120,
            "total_pharmacies": 100,
            "actual_promo_sales_units": Decimal("140"),
            "actual_investments": Decimal("9000"),
            "actual_promo_rub": Decimal("70000"),
            "actual_promo_uplift_units": Decimal("40"),
            "actual_promo_uplift_rub": Decimal("20000"),
            "actual_external_ecom_units": Decimal("10"),
            "actual_corrected_baseline": Decimal("100"),
            "olap_price": Decimal("480"),
            "conditions": "Секретные условия",
            "comments": "Секретный комментарий",
            "agreement1": "согласовано",
            "agreement1_status": "approved",
            "agreement1_comment": "реальный комментарий",
            "agreement2": None,
            "agreement2_status": "pending",
            "agreement2_comment": None,
            "id_directum": "123",
            "ds_number": "456",
            "date": date(2025, 5, 1),
            "created_by": "real_user",
            "updated_by": "real_user",
            "deleted_at": None,
            "baseline_rub": Decimal("50000"),
            "plan_promo_rub": Decimal("75000"),
            "plan_promo_uplift_units": Decimal("50"),
            "plan_promo_uplift_pct_units": Decimal("33.3333"),
            "plan_promo_uplift_rub": Decimal("25000"),
            "plan_promo_uplift_pct_rub": Decimal("33.3333"),
            "plan_investments_pct": Decimal("13.3333"),
            "plan_roi": Decimal("0"),
            "net_promo_uplift_rub": Decimal("8000"),
            "net_promo_uplift_pct": Decimal("11.4286"),
            "actual_investments_pct": Decimal("12.8571"),
            "actual_roi": Decimal("-11.1111"),
            "actual_promo_rub_wo_ecom": Decimal("65000"),
            "actual_promo_uplift_units_wo_ecom": Decimal("30"),
            "actual_promo_uplift_rub_wo_ecom": Decimal("15000"),
            "net_promo_uplift_rub_wo_ecom": Decimal("6000"),
            "net_promo_uplift_pct_wo_ecom": Decimal("9.2308"),
            "actual_investments_pct_wo_ecom": Decimal("13.8462"),
            "actual_roi_wo_ecom": Decimal("-33.3333"),
            "plan_vs_fact_rub": Decimal("93.3333"),
            "plan_vs_fact_investments": Decimal("90"),
            "turnover_per_point": Decimal("0.8333"),
            "turnover_per_point_promo": Decimal("1.1667"),
            "plan_promo_cip_olap": Decimal("72000"),
            "fact_promo_cip_olap": Decimal("67200"),
            "plan_promo_uplift_cip_olap": Decimal("24000"),
            "fact_promo_uplift_cip_olap": Decimal("19200"),
        }
        maps = build_entity_maps(
            self.synthetic,
            [source],
            [{"sku": source["sku"], "brand": source["brand"], "brand_as": source["brand_as"]}],
            [{"kam": source["kam"], "network_name": source["network_name"], "valid_from": date(2020, 1, 1)}],
        )
        transformed = transform_promo_row(source, 1, maps, self.synthetic)

        self.assertEqual(transformed["year"], 2025)
        self.assertEqual(transformed["month"], 5)
        self.assertEqual(transformed["quarter"], 2)
        self.assertEqual(transformed["mechanics"], "Скидка")
        self.assertTrue(transformed["network_name"].startswith("Демо-сеть"))
        self.assertTrue(transformed["sku"].startswith("DB01-SKU"))
        self.assertNotIn("Секрет", transformed["conditions"])
        self.assertNotIn("Секрет", transformed["comments"])
        self.assertEqual(transformed["agreement1"], "Согласовано в демонстрационном контуре.")
        self.assertEqual(transformed["created_by"], "demo_import")
        self.assertLessEqual(transformed["promo_pharmacies"], transformed["total_pharmacies"])
        self.assertNotEqual(transformed["plan_promo_units"], source["plan_promo_units"])

    def test_transform_is_deterministic(self):
        value = self.synthetic.factor("volume", "network|sku")
        self.assertEqual(value, self.synthetic.factor("volume", "network|sku"))


if __name__ == "__main__":
    unittest.main()
