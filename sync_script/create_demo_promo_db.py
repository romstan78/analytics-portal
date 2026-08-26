#!/usr/bin/env python3
"""Build an isolated promo demo dataset from the local SQL Server.

The source connection is SELECT-only. All writes are restricted to a database
whose name contains ``demo`` and whose server/database pair differs from the
source. Entity mappings and numeric changes are deterministic for one mapping
salt, while source values are never written to reports or mapping tables.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import math
import os
import re
import sys
import unicodedata
from collections import defaultdict
from datetime import date, datetime
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path
from typing import Any, Iterable

import pyodbc
from dotenv import dotenv_values

from dedupe_promo import analyze_rows, fetch_duplicate_rows, fetch_table_columns


PROMO_TABLE = "dbo.tbl_PromoActivities"
RESET_CONFIRMATION = "RESET_DEMO_PROMO_DB"

NETWORK_WORDS = (
    "Аврора", "Вектор", "Вершина", "Горизонт", "Дельта", "Импульс",
    "Каскад", "Клевер", "Контур", "Линия", "Маяк", "Меридиан",
    "Орион", "Орбита", "Парус", "Полюс", "Пульс", "Рассвет",
    "Север", "Сфера", "Фокус", "Янтарь", "Альта", "Берег",
    "Волна", "Гранит", "Доверие", "Единство", "Заря", "Исток",
    "Кедр", "Лотос", "Мост", "Навигатор", "Норд", "Опора",
    "Перспектива", "Платформа", "Поток", "Простор", "Ритм", "Рубеж",
    "Спектр", "Старт", "Тандем", "Темп", "Точка", "Траектория",
    "Флагман", "Формула", "Центр", "Элемент", "Энергия", "Баланс",
    "Вега", "Искра", "Мотив", "Прайм", "Сигма", "Кварц",
)

KAM_NAMES = (
    "Алексеева Марина", "Белов Андрей", "Волкова Ирина",
    "Громов Павел", "Данилова Елена", "Ершов Максим",
    "Жукова Ольга", "Крылов Сергей", "Миронова Анна",
    "Орлов Дмитрий", "Романова Татьяна", "Соколов Илья",
    "Тихонова Наталья", "Федоров Артем", "Чернова Юлия",
    "Широков Денис", "Яковлева Светлана", "Зайцев Михаил",
    "Лебедева Дарья", "Новиков Виктор",
)

BRAND_WORDS = (
    "Альфа", "Бета", "Гамма", "Дельта", "Вега", "Лира",
    "Омега", "Орион", "Сигма", "Кварц", "Пульс", "Фокус",
)

REGIONS = (
    "Демо-регион 01 «Центр»", "Демо-регион 02 «Северо-Запад»",
    "Демо-регион 03 «Юг»", "Демо-регион 04 «Поволжье»",
    "Демо-регион 05 «Урал»", "Демо-регион 06 «Сибирь»",
    "Демо-регион 07 «Дальний Восток»",
)

CONDITION_TEMPLATES = (
    "Демонстрационные условия промо. Данные сформированы для тестового контура.",
    "Синтетические условия акции для презентации аналитического портала.",
    "Демо-условия: параметры не относятся к реальным контрагентам.",
)

COMMENT_TEMPLATES = (
    "Синтетический комментарий для демонстрации.",
    "Демо-комментарий: требуется проверить расчётные показатели.",
    "Комментарий создан автоматически для тестового контура.",
)

AGREEMENT_TEMPLATES = {
    "approved": "Согласовано в демонстрационном контуре.",
    "rejected": "Отклонено: требуется корректировка демонстрационных параметров.",
    "commented": "Требуется уточнение демонстрационных параметров.",
    "pending": "Ожидает демонстрационного согласования.",
}

BASE_NUMERIC_FACTORS = {
    "volume": ("0.91", "0.95", "0.98", "1.02", "1.06", "1.10"),
    "period": ("0.98", "1.00", "1.03"),
    "actual": ("0.96", "0.99", "1.02", "1.04"),
    "price": ("0.93", "0.97", "1.01", "1.05", "1.08"),
    "investment": ("0.90", "0.95", "0.98", "1.03", "1.06", "1.10"),
    "pharmacy": ("0.92", "0.96", "1.01", "1.05", "1.08"),
    "ratio": ("0.96", "0.99", "1.02", "1.04"),
}

UNIT_BASE_FIELDS = (
    "baseline_units", "plan_promo_units", "max_sales_units",
)
ACTUAL_UNIT_FIELDS = (
    "actual_corrected_baseline", "actual_network_sales_units",
    "actual_promo_sales_units", "actual_promo_uplift_units",
    "actual_external_ecom_units",
)
MONEY_BASE_FIELDS = (
    "plan_investments_rub", "actual_investments", "discount_amount",
)
ACTUAL_MONEY_FIELDS = (
    "actual_promo_rub", "actual_promo_uplift_rub",
)
DERIVED_FIELDS = {
    "baseline_rub", "plan_promo_rub", "plan_promo_uplift_units",
    "plan_promo_uplift_pct_units", "plan_promo_uplift_rub",
    "plan_promo_uplift_pct_rub", "plan_investments_pct", "plan_roi",
    "net_promo_uplift_rub", "net_promo_uplift_pct",
    "actual_investments_pct", "actual_roi", "actual_promo_rub_wo_ecom",
    "actual_promo_uplift_units_wo_ecom", "actual_promo_uplift_rub_wo_ecom",
    "net_promo_uplift_rub_wo_ecom", "net_promo_uplift_pct_wo_ecom",
    "actual_investments_pct_wo_ecom", "actual_roi_wo_ecom",
    "plan_vs_fact_rub", "plan_vs_fact_investments",
    "turnover_per_point", "turnover_per_point_promo",
    "plan_promo_cip_olap", "fact_promo_cip_olap",
    "plan_promo_uplift_cip_olap", "fact_promo_uplift_cip_olap",
}


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Создать обезличенный промо-срез в изолированной demo-БД."
    )
    parser.add_argument("--env-file", type=Path, default=Path(".env"))
    parser.add_argument("--source-server", default="127.0.0.1,1433")
    parser.add_argument("--target-server", default="127.0.0.1,1434")
    parser.add_argument("--source-db", default=None)
    parser.add_argument("--target-db", default=None)
    parser.add_argument("--replace", action="store_true")
    parser.add_argument("--confirm", default="")
    return parser.parse_args(argv)


def clean_text(value: Any) -> str:
    if value is None:
        return ""
    text = unicodedata.normalize("NFKC", str(value))
    text = text.replace("\u00a0", " ").replace("\u200b", "").replace("\ufeff", "")
    return re.sub(r"\s+", " ", text).strip()


def canonical_text(value: Any) -> str:
    return clean_text(value).casefold().replace("ё", "е")


def canonical_entity(value: Any) -> str:
    return re.sub(r"[^0-9a-zа-я]+", "", canonical_text(value))


def canonical_kam(value: Any) -> str:
    tokens = re.findall(r"[a-zа-я]+(?:-[a-zа-я]+)?", canonical_text(value))
    if 2 <= len(tokens) <= 3:
        return "|".join(sorted(tokens))
    return canonical_entity(value)


def decimal_value(value: Any) -> Decimal | None:
    if value is None:
        return None
    if isinstance(value, Decimal):
        return value
    return Decimal(str(value))


def quantize_money(value: Decimal) -> Decimal:
    return value.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


def scale_decimal(value: Any, factor: Decimal, places: str = "0.01") -> Decimal | None:
    number = decimal_value(value)
    if number is None or number == 0:
        return number
    return (number * factor).quantize(Decimal(places), rounding=ROUND_HALF_UP)


def scale_int(value: Any, factor: Decimal) -> int | None:
    number = decimal_value(value)
    if number is None:
        return None
    if number == 0:
        return 0
    scaled = (number * factor).quantize(Decimal("1"), rounding=ROUND_HALF_UP)
    return int(scaled)


class StableSynthetic:
    def __init__(self, salt: str):
        if len(salt) < 16:
            raise ValueError("DEMO_MAPPING_SALT/JWT_SECRET должен быть не короче 16 символов")
        self.salt = salt.encode("utf-8")

    def digest(self, namespace: str, value: Any) -> bytes:
        payload = f"{namespace}|{value}".encode("utf-8")
        return hmac.new(self.salt, payload, hashlib.sha256).digest()

    def order(self, namespace: str, values: Iterable[str]) -> list[str]:
        return sorted(set(values), key=lambda value: self.digest(namespace, value))

    def choose(self, namespace: str, value: Any, choices: tuple[Any, ...]) -> Any:
        digest = self.digest(namespace, value)
        return choices[int.from_bytes(digest[:8], "big") % len(choices)]

    def factor(self, namespace: str, value: Any) -> Decimal:
        choices = BASE_NUMERIC_FACTORS[namespace]
        return Decimal(self.choose(namespace, value, choices))


def connect(server: str, database: str, password: str, readonly: bool) -> pyodbc.Connection:
    errors = []
    for driver in ("ODBC Driver 18 for SQL Server", "ODBC Driver 17 for SQL Server"):
        options = "ApplicationIntent=ReadOnly;" if readonly else ""
        connection_string = (
            f"DRIVER={{{driver}}};SERVER={server};DATABASE={database};"
            f"UID=sa;PWD={password};Encrypt=yes;TrustServerCertificate=yes;{options}"
        )
        try:
            connection = pyodbc.connect(connection_string, autocommit=False, timeout=30)
            cursor = connection.cursor()
            cursor.execute("SET NOCOUNT ON; SET XACT_ABORT ON;")
            if readonly:
                cursor.execute("SET TRANSACTION ISOLATION LEVEL READ UNCOMMITTED;")
            cursor.close()
            return connection
        except pyodbc.Error as error:
            errors.append(f"{driver}: {error.args[0] if error.args else 'connection error'}")
    raise RuntimeError("Не удалось подключиться к SQL Server: " + "; ".join(errors))


def fetch_dicts(cursor: pyodbc.Cursor, query: str, params: tuple[Any, ...] = ()) -> list[dict[str, Any]]:
    cursor.execute(query, params)
    names = [column[0] for column in cursor.description]
    return [dict(zip(names, row)) for row in cursor.fetchall()]


def source_fingerprint(cursor: pyodbc.Cursor) -> tuple[Any, ...]:
    cursor.execute(
        """
        SELECT COUNT_BIG(*), COALESCE(SUM(CAST(id AS BIGINT)),0),
               MAX(updated_at),
               CHECKSUM_AGG(BINARY_CHECKSUM(id, updated_at, deleted_at))
        FROM dbo.tbl_PromoActivities
        """
    )
    return tuple(cursor.fetchone())


def require_target_safety(
    source_server: str,
    source_db: str,
    target_server: str,
    target_db: str,
) -> None:
    if "demo" not in target_db.casefold():
        raise ValueError("Имя целевой базы обязано содержать 'demo'")
    if source_server.casefold() == target_server.casefold() and source_db.casefold() == target_db.casefold():
        raise ValueError("Источник и целевая база совпадают")


def table_count(cursor: pyodbc.Cursor, table: str) -> int:
    if not re.fullmatch(r"dbo\.tbl_[A-Za-z0-9_]+", table):
        raise ValueError(f"Небезопасное имя таблицы: {table}")
    cursor.execute(f"SELECT COUNT_BIG(*) FROM {table}")
    return int(cursor.fetchone()[0])


def build_exact_duplicate_map(cursor: pyodbc.Cursor) -> tuple[dict[int, int], dict[str, int]]:
    columns = fetch_table_columns(cursor)
    duplicate_rows = fetch_duplicate_rows(cursor, columns)
    groups = analyze_rows(duplicate_rows, columns)
    duplicate_to_keeper: dict[int, int] = {}
    exact_groups = 0
    for group in groups:
        if group["classification"] != "exact_duplicate":
            continue
        exact_groups += 1
        for duplicate_id in group["duplicate_ids"]:
            duplicate_to_keeper[int(duplicate_id)] = int(group["keeper_id"])
    return duplicate_to_keeper, {
        "business_key_groups": len(groups),
        "exact_duplicate_groups": exact_groups,
        "safe_exact_duplicate_rows": len(duplicate_to_keeper),
    }


def map_by_hmac(
    synthetic: StableSynthetic,
    namespace: str,
    keys: Iterable[str],
    label,
) -> dict[str, str]:
    ordered = synthetic.order(namespace, (key for key in keys if key))
    return {key: label(index, key) for index, key in enumerate(ordered, start=1)}


def build_entity_maps(
    synthetic: StableSynthetic,
    promo_rows: list[dict[str, Any]],
    sku_rows: list[dict[str, Any]],
    kam_rows: list[dict[str, Any]],
) -> dict[str, Any]:
    network_keys = {
        canonical_entity(row.get("network_name")) for row in promo_rows if row.get("network_name")
    }
    network_keys.update(
        canonical_entity(row.get("network_name")) for row in kam_rows if row.get("network_name")
    )
    network_map = map_by_hmac(
        synthetic,
        "network-order",
        network_keys,
        lambda index, _key: f"Демо-сеть {index:02d} «{NETWORK_WORDS[(index - 1) % len(NETWORK_WORDS)]}»",
    )

    kam_keys = {
        canonical_kam(row.get("kam")) for row in promo_rows if row.get("kam")
    }
    kam_keys.update(canonical_kam(row.get("kam")) for row in kam_rows if row.get("kam"))
    kam_map = map_by_hmac(
        synthetic,
        "kam-order",
        kam_keys,
        lambda index, _key: KAM_NAMES[index - 1]
        if index <= len(KAM_NAMES)
        else f"Демонстрационный КАМ {index:02d}",
    )

    brand_as_keys = {
        canonical_entity(row.get("brand_as"))
        for row in (*promo_rows, *sku_rows)
        if row.get("brand_as")
    }
    brand_as_map = map_by_hmac(
        synthetic,
        "brand-as-order",
        brand_as_keys,
        lambda index, _key: f"Демо-бренд {index:02d} «{BRAND_WORDS[(index - 1) % len(BRAND_WORDS)]}»",
    )

    brand_to_brand_as: dict[str, set[str]] = defaultdict(set)
    for row in (*sku_rows, *promo_rows):
        brand_key = canonical_entity(row.get("brand"))
        brand_as_key = canonical_entity(row.get("brand_as"))
        if brand_key and brand_as_key:
            brand_to_brand_as[brand_key].add(brand_as_key)

    brand_keys = set(brand_to_brand_as)
    brand_order = synthetic.order("brand-order", brand_keys)
    brand_line_counter: dict[str, int] = defaultdict(int)
    brand_map: dict[str, str] = {}
    for brand_key in brand_order:
        parent_keys = sorted(brand_to_brand_as[brand_key])
        parent_key = parent_keys[0] if parent_keys else ""
        if brand_key == parent_key and parent_key in brand_as_map:
            brand_map[brand_key] = brand_as_map[parent_key]
            continue
        parent_name = brand_as_map.get(parent_key, "Демо-бренд")
        brand_line_counter[parent_key] += 1
        brand_map[brand_key] = f"{parent_name} — Линия {brand_line_counter[parent_key]}"

    sku_parent: dict[str, str] = {}
    for row in (*sku_rows, *promo_rows):
        sku_key = canonical_entity(row.get("sku"))
        brand_as_key = canonical_entity(row.get("brand_as"))
        if sku_key and brand_as_key and sku_key not in sku_parent:
            sku_parent[sku_key] = brand_as_key

    sku_map: dict[str, str] = {}
    by_parent: dict[str, list[str]] = defaultdict(list)
    for sku_key, parent_key in sku_parent.items():
        by_parent[parent_key].append(sku_key)
    brand_as_order = {key: index for index, key in enumerate(synthetic.order("brand-as-order", brand_as_keys), 1)}
    for parent_key, sku_keys in by_parent.items():
        parent_index = brand_as_order.get(parent_key, 99)
        for sku_index, sku_key in enumerate(synthetic.order(f"sku-order:{parent_key}", sku_keys), 1):
            sku_map[sku_key] = f"DB{parent_index:02d}-SKU{sku_index:03d}"

    return {
        "network": network_map,
        "kam": kam_map,
        "brand": brand_map,
        "brand_as": brand_as_map,
        "sku": sku_map,
    }


def calculated_value(row: dict[str, Any], column: str, value: Decimal | int | date | None) -> Any:
    return value if row.get(column) is not None else None


def safe_div(numerator: Decimal, denominator: Decimal) -> Decimal:
    return Decimal("0") if denominator == 0 else numerator / denominator


def transform_promo_row(
    source: dict[str, Any],
    demo_id: int,
    maps: dict[str, Any],
    synthetic: StableSynthetic,
) -> dict[str, Any]:
    row = dict(source)
    network_key = canonical_entity(source.get("network_name"))
    kam_key = canonical_kam(source.get("kam"))
    brand_key = canonical_entity(source.get("brand"))
    brand_as_key = canonical_entity(source.get("brand_as"))
    sku_key = canonical_entity(source.get("sku"))
    business_key = "|".join(
        str(source.get(name) or "")
        for name in ("network_name", "sku", "year", "month", "mechanics", "id")
    )

    row["id"] = demo_id
    row["network_name"] = maps["network"].get(network_key) if network_key else None
    row["kam"] = maps["kam"].get(kam_key) if kam_key else None
    row["brand"] = maps["brand"].get(brand_key) if brand_key else None
    row["brand_as"] = maps["brand_as"].get(brand_as_key) if brand_as_key else None
    row["sku"] = maps["sku"].get(sku_key) if sku_key else None
    if source.get("key_region") is not None:
        row["key_region"] = synthetic.choose("network-region", network_key, REGIONS)
    if source.get("top20_segment") is not None:
        row["top20_segment"] = synthetic.choose(
            "network-segment", network_key, ("Демо TOP-20", "Демо региональная")
        )

    volume_factor = synthetic.factor("volume", f"{network_key}|{sku_key}")
    period_factor = synthetic.factor("period", f"{source.get('year')}|{source.get('month')}")
    actual_factor = synthetic.factor("actual", business_key)
    price_factor = synthetic.factor("price", sku_key)
    investment_factor = synthetic.factor("investment", f"{network_key}|{source.get('mechanics')}")
    pharmacy_factor = synthetic.factor("pharmacy", network_key)
    ratio_factor = synthetic.factor("ratio", business_key)
    plan_unit_factor = volume_factor * period_factor
    actual_unit_factor = plan_unit_factor * actual_factor

    for field in UNIT_BASE_FIELDS:
        row[field] = scale_decimal(source.get(field), plan_unit_factor)
    for field in ACTUAL_UNIT_FIELDS:
        row[field] = scale_decimal(source.get(field), actual_unit_factor)
    for field in MONEY_BASE_FIELDS:
        row[field] = scale_decimal(source.get(field), investment_factor)
    for field in ACTUAL_MONEY_FIELDS:
        row[field] = scale_decimal(source.get(field), actual_unit_factor * price_factor)

    row["contract_price"] = scale_decimal(source.get("contract_price"), price_factor)
    row["olap_price"] = scale_decimal(source.get("olap_price"), price_factor)
    row["category_dynamics"] = scale_decimal(source.get("category_dynamics"), ratio_factor)

    gm = decimal_value(source.get("gm"))
    if gm is not None and gm != 0:
        delta = synthetic.choose("gm-delta", business_key, (Decimal("-0.02"), Decimal("-0.01"), Decimal("0.01"), Decimal("0.02")))
        gm = max(Decimal("0.01"), min(Decimal("0.99"), gm + delta))
        gm = gm.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    row["gm"] = gm

    total_pharmacies = scale_int(source.get("total_pharmacies"), pharmacy_factor)
    promo_pharmacies = scale_int(source.get("promo_pharmacies"), pharmacy_factor)
    if total_pharmacies is not None and promo_pharmacies is not None:
        promo_pharmacies = min(promo_pharmacies, total_pharmacies)
    row["total_pharmacies"] = total_pharmacies
    row["promo_pharmacies"] = promo_pharmacies

    if source.get("id_directum") is not None:
        row["id_directum"] = f"DEMO-DIR-{demo_id:06d}"
    if source.get("ds_number") is not None:
        year = int(source.get("year") or 0)
        row["ds_number"] = f"DEMO-DS-{year:04d}-{demo_id:06d}"

    template_key = f"promo-text|{demo_id}"
    if clean_text(source.get("conditions")):
        row["conditions"] = synthetic.choose("conditions", template_key, CONDITION_TEMPLATES)
    elif source.get("conditions") is not None:
        row["conditions"] = None
    if clean_text(source.get("comments")):
        row["comments"] = synthetic.choose("comments", template_key, COMMENT_TEMPLATES)
    elif source.get("comments") is not None:
        row["comments"] = None

    for legacy, status_column, comment_column in (
        ("agreement1", "agreement1_status", "agreement1_comment"),
        ("agreement2", "agreement2_status", "agreement2_comment"),
    ):
        status = clean_text(source.get(status_column)).casefold() or "pending"
        template = AGREEMENT_TEMPLATES.get(status, AGREEMENT_TEMPLATES["commented"])
        if clean_text(source.get(legacy)):
            row[legacy] = template
        elif source.get(legacy) is not None:
            row[legacy] = None
        if clean_text(source.get(comment_column)):
            row[comment_column] = template
        elif source.get(comment_column) is not None:
            row[comment_column] = None

    if source.get("created_by") is not None:
        row["created_by"] = "demo_import"
    if source.get("updated_by") is not None:
        row["updated_by"] = "demo_import"

    if row.get("month") is not None:
        month = int(row["month"])
        row["quarter"] = calculated_value(source, "quarter", ((month - 1) // 3) + 1)
        if row.get("year") is not None:
            row["date"] = calculated_value(source, "date", date(int(row["year"]), month, 1))

    baseline_units = decimal_value(row.get("baseline_units")) or Decimal("0")
    plan_units = decimal_value(row.get("plan_promo_units")) or Decimal("0")
    plan_investments = decimal_value(row.get("plan_investments_rub")) or Decimal("0")
    contract_price = decimal_value(row.get("contract_price")) or Decimal("0")
    gm_value = decimal_value(row.get("gm")) or Decimal("0")
    actual_units = decimal_value(row.get("actual_promo_sales_units")) or Decimal("0")
    actual_investments = decimal_value(row.get("actual_investments")) or Decimal("0")
    actual_promo_rub = decimal_value(row.get("actual_promo_rub")) or Decimal("0")
    actual_uplift_units = decimal_value(row.get("actual_promo_uplift_units")) or Decimal("0")
    actual_uplift_rub = decimal_value(row.get("actual_promo_uplift_rub")) or Decimal("0")
    external_ecom = decimal_value(row.get("actual_external_ecom_units")) or Decimal("0")
    corrected_baseline = decimal_value(row.get("actual_corrected_baseline")) or Decimal("0")
    olap_price = decimal_value(row.get("olap_price")) or Decimal("0")
    promo_points = Decimal(row.get("promo_pharmacies") or 1)

    plan_promo_rub = plan_units * contract_price
    plan_uplift_units = plan_units - baseline_units
    plan_uplift_rub = plan_uplift_units * contract_price
    net_uplift_rub = actual_uplift_rub * gm_value
    actual_promo_rub_wo_ecom = actual_promo_rub - external_ecom * contract_price
    actual_uplift_units_wo_ecom = actual_uplift_units - external_ecom
    actual_uplift_rub_wo_ecom = actual_uplift_units_wo_ecom * contract_price
    net_uplift_rub_wo_ecom = actual_uplift_rub_wo_ecom * gm_value

    computed = {
        "baseline_rub": baseline_units * contract_price,
        "plan_promo_rub": plan_promo_rub,
        "plan_promo_uplift_units": plan_uplift_units,
        "plan_promo_uplift_pct_units": safe_div(plan_uplift_units, plan_units) * 100,
        "plan_promo_uplift_rub": plan_uplift_rub,
        "plan_promo_uplift_pct_rub": safe_div(plan_uplift_rub, plan_promo_rub) * 100,
        "plan_investments_pct": safe_div(plan_investments, plan_promo_rub) * 100,
        "plan_roi": safe_div(plan_uplift_rub, plan_investments) * gm_value * 100 - (Decimal("100") if plan_investments != 0 else Decimal("0")),
        "net_promo_uplift_rub": net_uplift_rub,
        "net_promo_uplift_pct": safe_div(net_uplift_rub, actual_promo_rub) * 100,
        "actual_investments_pct": safe_div(actual_investments, actual_promo_rub) * 100,
        "actual_roi": safe_div(actual_uplift_rub, actual_investments) * gm_value * 100 - (Decimal("100") if actual_investments != 0 else Decimal("0")),
        "actual_promo_rub_wo_ecom": actual_promo_rub_wo_ecom,
        "actual_promo_uplift_units_wo_ecom": actual_uplift_units_wo_ecom,
        "actual_promo_uplift_rub_wo_ecom": actual_uplift_rub_wo_ecom,
        "net_promo_uplift_rub_wo_ecom": net_uplift_rub_wo_ecom,
        "net_promo_uplift_pct_wo_ecom": safe_div(net_uplift_rub_wo_ecom, actual_promo_rub_wo_ecom) * 100,
        "actual_investments_pct_wo_ecom": safe_div(actual_investments, actual_promo_rub_wo_ecom) * 100,
        "actual_roi_wo_ecom": safe_div(actual_uplift_rub_wo_ecom, actual_investments) * gm_value * 100 - (Decimal("100") if actual_investments != 0 else Decimal("0")),
        "plan_vs_fact_rub": safe_div(actual_promo_rub, plan_promo_rub) * 100,
        "plan_vs_fact_investments": safe_div(actual_investments, plan_investments) * 100,
        "turnover_per_point": corrected_baseline / promo_points,
        "turnover_per_point_promo": actual_units / promo_points,
        "plan_promo_cip_olap": plan_units * olap_price,
        "fact_promo_cip_olap": actual_units * olap_price,
        "plan_promo_uplift_cip_olap": plan_uplift_units * olap_price,
        "fact_promo_uplift_cip_olap": actual_uplift_units * olap_price,
    }
    for field, value in computed.items():
        row[field] = calculated_value(source, field, quantize_money(value))

    return row


def execute_many(cursor: pyodbc.Cursor, query: str, values: list[tuple[Any, ...]], batch_size: int = 250) -> None:
    cursor.fast_executemany = False
    for start in range(0, len(values), batch_size):
        cursor.executemany(query, values[start:start + batch_size])


def clear_target(cursor: pyodbc.Cursor) -> None:
    # Интернет-продажи и реестр сетей очищаются вместе с промо: они собраны из
    # демо-сетей, брендов и SKU, а новая загрузка может выдать другие
    # соответствия. Иначе в demo-БД остались бы продажи и планы по
    # несуществующим сетям. Порядок учитывает внешние ключи реестра.
    for table in (
        "dbo.tbl_NetworkContractPriceExclusions",
        "dbo.tbl_NetworkContractPrices",
        "dbo.tbl_NetworkPeriodGroups",
        "dbo.tbl_NetworkForecasts",
        "dbo.tbl_NetworkMonthlyFacts",
        "dbo.tbl_NetworkComments",
        "dbo.tbl_NetworkPlans",
        "dbo.tbl_NetworkPeriods",
        "dbo.tbl_Networks",
        "dbo.tbl_EcomSalesNormalized",
        "dbo.tbl_EcomSalesConsolidated",
        "dbo.tbl_ChannelSegmentMapping",
        "dbo.tbl_PromoDedupRelatedMoves",
        "dbo.tbl_PromoDedupChanges",
        "dbo.tbl_PromoDedupRuns",
        "dbo.tbl_PromoComments",
        "dbo.tbl_AuditLog",
        "dbo.tbl_PromoActivities",
        "dbo.tbl_KAMNetworkMapping",
        "dbo.tbl_SKUMapping",
        "dbo.tbl_MechanicsChannelMapping",
        "dbo.tbl_NetworkGeoMapping",
    ):
        cursor.execute(f"DELETE FROM {table}")


def target_has_data(cursor: pyodbc.Cursor) -> bool:
    tables = (
        "dbo.tbl_PromoActivities", "dbo.tbl_SKUMapping",
        "dbo.tbl_KAMNetworkMapping", "dbo.tbl_MechanicsChannelMapping",
        "dbo.tbl_NetworkGeoMapping",
    )
    return any(table_count(cursor, table) > 0 for table in tables)


def insert_reference_data(
    cursor: pyodbc.Cursor,
    maps: dict[str, Any],
    synthetic: StableSynthetic,
    sku_rows: list[dict[str, Any]],
    kam_rows: list[dict[str, Any]],
    geo_rows: list[dict[str, Any]],
    mechanics_rows: list[dict[str, Any]],
    promo_rows: list[dict[str, Any]],
) -> dict[str, int]:
    sku_values = []
    seen_skus = set()
    for source in sku_rows:
        sku_key = canonical_entity(source.get("sku"))
        if not sku_key or sku_key in seen_skus:
            continue
        seen_skus.add(sku_key)
        sku_values.append((
            maps["sku"][sku_key],
            maps["brand"].get(canonical_entity(source.get("brand"))),
            maps["brand_as"].get(canonical_entity(source.get("brand_as"))),
        ))
    execute_many(
        cursor,
        "INSERT INTO dbo.tbl_SKUMapping(sku,brand,brand_as) VALUES (?,?,?)",
        sku_values,
    )

    pair_dates: dict[tuple[str, str], date] = {}
    for source in kam_rows:
        kam_key = canonical_kam(source.get("kam"))
        network_key = canonical_entity(source.get("network_name"))
        valid_from = source.get("valid_from")
        if not kam_key or not network_key or valid_from is None:
            continue
        pair = (kam_key, network_key)
        pair_dates[pair] = min(pair_dates.get(pair, valid_from), valid_from)

    for source in promo_rows:
        if source.get("deleted_at") is not None:
            continue
        kam_key = canonical_kam(source.get("kam"))
        network_key = canonical_entity(source.get("network_name"))
        if not kam_key or not network_key:
            continue
        fallback = date(int(source.get("year") or 2000), 1, 1)
        pair_dates.setdefault((kam_key, network_key), fallback)

    kam_values = [
        (maps["kam"][kam_key], maps["network"][network_key], valid_from)
        for (kam_key, network_key), valid_from in sorted(pair_dates.items())
        if kam_key in maps["kam"] and network_key in maps["network"]
    ]
    execute_many(
        cursor,
        "INSERT INTO dbo.tbl_KAMNetworkMapping(kam,network_name,valid_from) VALUES (?,?,?)",
        kam_values,
    )

    mechanics_values = []
    seen_mechanics = set()
    for source in mechanics_rows:
        mechanics = clean_text(source.get("mechanics"))
        if not mechanics or mechanics in seen_mechanics:
            continue
        seen_mechanics.add(mechanics)
        mechanics_values.append((mechanics, source.get("channel")))
    execute_many(
        cursor,
        "INSERT INTO dbo.tbl_MechanicsChannelMapping(mechanics,channel) VALUES (?,?)",
        mechanics_values,
    )

    geo_by_network = {
        canonical_entity(source.get("network_name")): source
        for source in geo_rows if source.get("network_name")
    }
    kam_by_network: dict[str, str] = {}
    for kam_key, network_key in pair_dates:
        kam_by_network.setdefault(network_key, kam_key)

    active_networks = {
        canonical_entity(source.get("network_name"))
        for source in promo_rows
        if source.get("deleted_at") is None and source.get("network_name")
    }
    geo_values = []
    for network_key in synthetic.order("network-order", active_networks):
        source = geo_by_network.get(network_key, {})
        kam_key = kam_by_network.get(network_key)
        fake_region = synthetic.choose("network-region", network_key, REGIONS)
        fake_segment = synthetic.choose(
            "network-segment", network_key, ("Демо TOP-20", "Демо региональная")
        )
        geo_values.append((
            maps["network"][network_key],
            maps["kam"].get(kam_key),
            source.get("network_type") or "Аптечная сеть",
            fake_segment,
            fake_region,
        ))
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_NetworkGeoMapping
            (network_name,kam,network_type,top20_segment,key_region)
        VALUES (?,?,?,?,?)
        """,
        geo_values,
    )

    return {
        "sku_mapping": len(sku_values),
        "kam_network_mapping": len(kam_values),
        "mechanics_mapping": len(mechanics_values),
        "network_geo_mapping": len(geo_values),
    }


def insert_promo_rows(
    cursor: pyodbc.Cursor,
    columns: list[str],
    rows: list[dict[str, Any]],
) -> None:
    quoted = ",".join(f"[{column}]" for column in columns)
    placeholders = ",".join("?" for _ in columns)
    values = [tuple(row.get(column) for column in columns) for row in rows]
    cursor.execute("SET IDENTITY_INSERT dbo.tbl_PromoActivities ON")
    try:
        execute_many(
            cursor,
            f"INSERT INTO dbo.tbl_PromoActivities ({quoted}) VALUES ({placeholders})",
            values,
            batch_size=100,
        )
    finally:
        cursor.execute("SET IDENTITY_INSERT dbo.tbl_PromoActivities OFF")


def insert_synthetic_history(
    cursor: pyodbc.Cursor,
    source_cursor: pyodbc.Cursor,
    source_to_demo_id: dict[int, int],
    duplicate_to_keeper: dict[int, int],
) -> dict[str, int]:
    comments = fetch_dicts(
        source_cursor,
        "SELECT promo_id,role,created_at FROM dbo.tbl_PromoComments ORDER BY id",
    )
    comment_values = []
    for index, source in enumerate(comments, 1):
        promo_id = int(source["promo_id"])
        canonical_source_id = duplicate_to_keeper.get(promo_id, promo_id)
        demo_id = source_to_demo_id.get(canonical_source_id)
        if demo_id is None:
            continue
        comment_values.append((
            demo_id,
            f"demo_user_{(index - 1) % 3 + 1:02d}",
            source.get("role") or "demo",
            COMMENT_TEMPLATES[(index - 1) % len(COMMENT_TEMPLATES)],
            source.get("created_at") or datetime.now(),
        ))
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_PromoComments
            (promo_id,user_name,role,comment_text,created_at)
        VALUES (?,?,?,?,?)
        """,
        comment_values,
    )

    audits = fetch_dicts(
        source_cursor,
        """
        SELECT entity_id,entity_type,action_type,created_at
        FROM dbo.tbl_AuditLog
        WHERE entity_type='promo'
        ORDER BY id
        """,
    )
    audit_values = []
    for index, source in enumerate(audits, 1):
        entity_id = int(source["entity_id"])
        canonical_source_id = duplicate_to_keeper.get(entity_id, entity_id)
        demo_id = source_to_demo_id.get(canonical_source_id)
        if demo_id is None:
            continue
        payload = json.dumps(
            {"demo": True, "note": "Синтетическая история изменения"},
            ensure_ascii=False,
        )
        audit_values.append((
            "promo",
            demo_id,
            f"demo_user_{(index - 1) % 3 + 1:02d}",
            source.get("action_type") or "UPDATE",
            payload,
            source.get("created_at") or datetime.now(),
        ))
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_AuditLog
            (entity_type,entity_id,user_name,action_type,changed_fields,created_at)
        VALUES (?,?,?,?,?,?)
        """,
        audit_values,
    )
    return {"promo_comments": len(comment_values), "audit_rows": len(audit_values)}


def verify_target(
    cursor: pyodbc.Cursor,
    maps: dict[str, Any],
    expected_rows: int,
    expected_years: set[int],
    expected_fact_ready: int,
) -> dict[str, Any]:
    cursor.execute(
        """
        SELECT COUNT_BIG(*),
               COUNT(DISTINCT network_name), COUNT(DISTINCT kam),
               COUNT(DISTINCT brand_as), COUNT(DISTINCT sku),
               COUNT(DISTINCT mechanics),
               SUM(CASE WHEN deleted_at IS NULL
                         AND actual_promo_sales_units IS NOT NULL
                         AND actual_investments IS NOT NULL THEN 1 ELSE 0 END),
               SUM(CASE WHEN deleted_at IS NULL
                         AND promo_pharmacies IS NOT NULL
                         AND total_pharmacies IS NOT NULL
                         AND promo_pharmacies>total_pharmacies THEN 1 ELSE 0 END)
        FROM dbo.tbl_PromoActivities
        """
    )
    row = cursor.fetchone()
    if int(row[0]) != expected_rows:
        raise RuntimeError(f"Неверное число промо-строк в demo-БД: {row[0]} != {expected_rows}")
    if int(row[6] or 0) != expected_fact_ready:
        raise RuntimeError("Изменилась структура покрытия фактическими данными")
    if int(row[7] or 0) != 0:
        raise RuntimeError("В demo-БД остались строки promo_pharmacies > total_pharmacies")

    cursor.execute("SELECT DISTINCT [year] FROM dbo.tbl_PromoActivities WHERE [year] IS NOT NULL")
    target_years = {int(item[0]) for item in cursor.fetchall()}
    if target_years != expected_years:
        raise RuntimeError(f"Набор годов изменился: {target_years} != {expected_years}")

    cursor.execute(
        """
        SELECT
          SUM(CASE WHEN network_name NOT LIKE N'Демо-сеть %' THEN 1 ELSE 0 END),
          SUM(CASE WHEN sku NOT LIKE N'DB%-SKU%' THEN 1 ELSE 0 END),
          SUM(CASE WHEN NULLIF(LTRIM(RTRIM(conditions)),N'') IS NOT NULL AND conditions NOT LIKE N'%демонстрац%'
                    AND conditions NOT LIKE N'%Синтетическ%'
                    AND conditions NOT LIKE N'%Демо-%' THEN 1 ELSE 0 END),
          SUM(CASE WHEN NULLIF(LTRIM(RTRIM(comments)),N'') IS NOT NULL AND comments NOT LIKE N'%демонстрац%'
                    AND comments NOT LIKE N'%Демо-%'
                    AND comments NOT LIKE N'%автоматически%' THEN 1 ELSE 0 END)
        FROM dbo.tbl_PromoActivities
        """
    )
    unsafe = cursor.fetchone()
    unsafe_counts = tuple(int(value or 0) for value in unsafe)
    if any(unsafe_counts):
        raise RuntimeError(
            "Проверка синтетических строк demo-БД не пройдена "
            f"(network={unsafe_counts[0]}, sku={unsafe_counts[1]}, "
            f"conditions={unsafe_counts[2]}, comments={unsafe_counts[3]})"
        )

    return {
        "promo_rows": int(row[0]),
        "networks": int(row[1]),
        "kams": int(row[2]),
        "brands_as": int(row[3]),
        "skus": int(row[4]),
        "mechanics": int(row[5]),
        "fact_ready_rows": int(row[6] or 0),
        "years_preserved": sorted(target_years),
        "entity_maps": {name: len(value) for name, value in maps.items()},
    }


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    env = dotenv_values(args.env_file)
    password = env.get("SA_PASSWORD") or os.getenv("SA_PASSWORD")
    source_db = args.source_db or env.get("DB_NAME") or "local_project_db"
    target_db = args.target_db or env.get("DEMO_DB_NAME") or "local_project_demo_db"
    salt = env.get("DEMO_MAPPING_SALT") or env.get("JWT_SECRET") or os.getenv("DEMO_MAPPING_SALT")
    if not password:
        raise RuntimeError("SA_PASSWORD не задан")
    if not salt:
        raise RuntimeError("DEMO_MAPPING_SALT или JWT_SECRET не задан")
    if args.replace and args.confirm != RESET_CONFIRMATION:
        raise ValueError(f"Для --replace требуется --confirm {RESET_CONFIRMATION}")
    require_target_safety(args.source_server, source_db, args.target_server, target_db)

    synthetic = StableSynthetic(salt)
    source = connect(args.source_server, source_db, password, readonly=True)
    target = connect(args.target_server, target_db, password, readonly=False)
    source_cursor = source.cursor()
    target_cursor = target.cursor()

    try:
        before = source_fingerprint(source_cursor)
        columns = fetch_table_columns(source_cursor)
        target_columns = fetch_table_columns(target_cursor)
        if columns != target_columns:
            raise RuntimeError("Схема tbl_PromoActivities различается между source и demo")

        duplicate_to_keeper, duplicate_summary = build_exact_duplicate_map(source_cursor)
        promo_rows = fetch_dicts(
            source_cursor,
            f"SELECT * FROM {PROMO_TABLE} WHERE deleted_at IS NULL ORDER BY id",
        )
        promo_rows = [row for row in promo_rows if int(row["id"]) not in duplicate_to_keeper]
        sku_rows = fetch_dicts(source_cursor, "SELECT sku,brand,brand_as FROM dbo.tbl_SKUMapping ORDER BY id")
        kam_rows = fetch_dicts(source_cursor, "SELECT kam,network_name,valid_from FROM dbo.tbl_KAMNetworkMapping ORDER BY id")
        geo_rows = fetch_dicts(source_cursor, "SELECT network_name,kam,network_type,top20_segment,key_region FROM dbo.tbl_NetworkGeoMapping ORDER BY id")
        mechanics_rows = fetch_dicts(source_cursor, "SELECT mechanics,channel FROM dbo.tbl_MechanicsChannelMapping ORDER BY id")

        maps = build_entity_maps(synthetic, promo_rows, sku_rows, kam_rows)
        source_order = sorted(
            promo_rows,
            key=lambda row: synthetic.digest("promo-id-order", int(row["id"])),
        )
        source_to_demo_id = {
            int(row["id"]): index for index, row in enumerate(source_order, start=1)
        }
        transformed = [
            transform_promo_row(row, source_to_demo_id[int(row["id"])], maps, synthetic)
            for row in source_order
        ]

        expected_fact_ready = sum(
            1 for row in transformed
            if row.get("deleted_at") is None
            and row.get("actual_promo_sales_units") is not None
            and row.get("actual_investments") is not None
        )
        expected_years = {int(row["year"]) for row in transformed if row.get("year") is not None}

        if target_has_data(target_cursor):
            if not args.replace:
                raise RuntimeError(
                    "Demo-БД уже содержит промо-данные. Используйте --replace с защитной фразой."
                )
            clear_target(target_cursor)

        reference_summary = insert_reference_data(
            target_cursor, maps, synthetic, sku_rows, kam_rows,
            geo_rows, mechanics_rows, promo_rows,
        )
        insert_promo_rows(target_cursor, columns, transformed)
        history_summary = insert_synthetic_history(
            target_cursor, source_cursor, source_to_demo_id, duplicate_to_keeper,
        )
        verification = verify_target(
            target_cursor, maps, len(transformed), expected_years, expected_fact_ready,
        )
        after = source_fingerprint(source_cursor)
        if before != after:
            raise RuntimeError("Контрольный отпечаток исходной промо-таблицы изменился")
        target.commit()
        source.rollback()

        print(json.dumps({
            "status": "ok",
            "source_unchanged": True,
            "target_database": target_db,
            "deduplication": duplicate_summary,
            "references": reference_summary,
            "synthetic_history": history_summary,
            "verification": verification,
        }, ensure_ascii=False, indent=2))
        return 0
    except Exception:
        target.rollback()
        source.rollback()
        raise
    finally:
        target_cursor.close()
        source_cursor.close()
        target.close()
        source.close()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"Ошибка создания demo-БД: {error}", file=sys.stderr)
        raise SystemExit(1)
