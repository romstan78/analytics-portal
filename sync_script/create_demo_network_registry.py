#!/usr/bin/env python3
"""Наполнить реестр сетей демонстрационными данными.

Как и загрузчик интернет-продаж, рабочая БД здесь не участвует: карточки сетей
собираются из демо-справочников, а факт отгрузок берётся из уже загруженных
интернет-продаж — из сегмента OLAP NW, который и означает отгрузки. Поэтому
реестр, промо и продажи показывают одни и те же сети с одной и той же
динамикой: сеть, теряющая объём на дашборде, теряет его и в плане реестра.

План считается от факта, а не наоборот: квартальный план получается делением
факта на коэффициент выполнения, поэтому часть кварталов перевыполнена, часть
провалена, и форма «план / факт» не выглядит нарисованной под одну гребёнку.

Помесячный факт сводится в квартальные fact_rub и fact_investments_rub плана
тем же запросом, что и рабочий загрузчик (import_network_facts.ROLLUP_SQL).
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from collections import defaultdict
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path
from typing import Any, Sequence

from dotenv import dotenv_values

from create_demo_promo_db import (
    StableSynthetic,
    clean_text,
    connect,
    execute_many,
    fetch_dicts,
    quantize_money,
    table_count,
)
from create_demo_ecom_sales import pick_decimal, unit_fraction
# Свод месячного факта в квартал берём готовым у рабочего загрузчика: вкладка
# «План и факт» читает fact_rub прямо из tbl_NetworkPlans, и собственный запрос
# здесь рано или поздно разошёлся бы с тем, что считает продакшн.
from import_network_facts import ROLLUP_SQL


RESET_CONFIRMATION = "RESET_DEMO_NETWORK_REGISTRY"

# Порядок важен: сначала всё, что ссылается на сеть, потом сама сеть.
REGISTRY_TABLES = (
    "dbo.tbl_NetworkContractPriceExclusions",
    "dbo.tbl_NetworkContractPrices",
    "dbo.tbl_NetworkPeriodGroups",
    "dbo.tbl_NetworkForecasts",
    "dbo.tbl_NetworkMonthlyFacts",
    "dbo.tbl_NetworkComments",
    "dbo.tbl_NetworkPlans",
    "dbo.tbl_NetworkPeriods",
    "dbo.tbl_Networks",
)

# Тип сети в реестре — классификатор без влияния на расчёты. Объединения аптек
# ведутся складским контрактом, остальные — обычным.
WAREHOUSE_GEO_TYPES = ("Group Pharm",)

# Распределение квартального плана по месяцам. Сумма каждой тройки — ровно 100:
# это требование CK_Networks_month_distribution.
MONTH_DISTRIBUTIONS = (
    (Decimal("30"), Decimal("30"), Decimal("40")),
    (Decimal("25"), Decimal("35"), Decimal("40")),
    (Decimal("33"), Decimal("33"), Decimal("34")),
    (Decimal("30"), Decimal("35"), Decimal("35")),
    (Decimal("20"), Decimal("40"), Decimal("40")),
)

# Процент промо-инвестиций считается от промо-оборота, а процент реестра — от
# всего объёма контракта, поэтому промо-значение сюда переносится с понижением.
PROMO_INVESTMENT_SHARE = Decimal("0.4")
INVESTMENT_PCT_BOUNDS = (Decimal("4"), Decimal("18"))
DEFAULT_INVESTMENT_PCT = Decimal("10")

# Коэффициент выполнения плана: факт / план. Разброс в обе стороны даёт форме
# и перевыполненные, и проваленные кварталы.
ACHIEVEMENT_BOUNDS = ("0.88", "1.14")
# Рост плана следующего года к факту текущего.
PLAN_GROWTH_BOUNDS = ("0.96", "1.18")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Наполнить реестр сетей demo-БД карточками, периодами, планами и фактом."
    )
    parser.add_argument("--env-file", type=Path, default=Path(".env"))
    parser.add_argument("--target-server", default="127.0.0.1,1434")
    parser.add_argument("--target-db", default=None)
    parser.add_argument("--replace", action="store_true")
    parser.add_argument("--confirm", default="")
    return parser.parse_args(argv)


def require_demo_target(target_db: str) -> None:
    if "demo" not in target_db.casefold():
        raise ValueError("Имя целевой базы обязано содержать 'demo'")


def quarter_of(month: int) -> int:
    return (month - 1) // 3 + 1


# ─── Чтение демо-справочников ──────────────────────────────────────────────

def fetch_networks(cursor) -> list[dict[str, Any]]:
    return fetch_dicts(
        cursor,
        """
        SELECT network_name, kam, network_type, top20_segment
        FROM dbo.tbl_NetworkGeoMapping
        WHERE network_name IS NOT NULL
        ORDER BY network_name
        """,
    )


def fetch_sku_brands(cursor) -> dict[str, str]:
    rows = fetch_dicts(cursor, "SELECT sku, brand, brand_as FROM dbo.tbl_SKUMapping ORDER BY sku")
    brands: dict[str, str] = {}
    for row in rows:
        sku = clean_text(row.get("sku"))
        if sku:
            brands[sku] = clean_text(row.get("brand_as")) or clean_text(row.get("brand")) or "Демо-бренд"
    return brands


def fetch_ecom_years(cursor) -> tuple[int, int]:
    cursor.execute(
        "SELECT MIN([year]), MAX([year]) FROM dbo.tbl_EcomSalesNormalized WHERE segment = N'OLAP NW'"
    )
    row = cursor.fetchone()
    if row is None or row[0] is None:
        raise RuntimeError(
            "В demo-БД нет интернет-продаж; сначала выполните make demo-ecom-load"
        )
    return int(row[0]), int(row[1])


def fetch_shipment_facts(cursor, year_from: int, year_to: int) -> list[dict[str, Any]]:
    """Отгрузки по сети, SKU и месяцу — сегмент OLAP NW интернет-продаж."""
    return fetch_dicts(
        cursor,
        """
        SELECT networkName, productName, [year], [month],
               SUM(CASE WHEN un_rub = N'уп'  THEN metric_value ELSE 0 END) AS units,
               SUM(CASE WHEN un_rub = N'руб' THEN metric_value ELSE 0 END) AS rub
        FROM dbo.tbl_EcomSalesNormalized
        WHERE segment = N'OLAP NW' AND [year] BETWEEN ? AND ?
        GROUP BY networkName, productName, [year], [month]
        """,
        (year_from, year_to),
    )


def fetch_investment_pct(cursor) -> dict[str, Decimal]:
    rows = fetch_dicts(
        cursor,
        """
        SELECT network_name, AVG(plan_investments_pct) AS pct
        FROM dbo.tbl_PromoActivities
        WHERE deleted_at IS NULL AND plan_investments_pct > 0
        GROUP BY network_name
        """,
    )
    result: dict[str, Decimal] = {}
    for row in rows:
        network = clean_text(row.get("network_name"))
        if not network or row.get("pct") is None:
            continue
        value = Decimal(str(row["pct"])) * PROMO_INVESTMENT_SHARE
        value = min(max(value, INVESTMENT_PCT_BOUNDS[0]), INVESTMENT_PCT_BOUNDS[1])
        result[network] = value.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    return result


# ─── Карточки сетей ────────────────────────────────────────────────────────

def build_network_rows(
    synthetic: StableSynthetic,
    networks: list[dict[str, Any]],
) -> list[tuple[Any, ...]]:
    rows: list[tuple[Any, ...]] = []
    for source in networks:
        name = clean_text(source.get("network_name"))
        if not name:
            continue
        kam = clean_text(source.get("kam")) or None
        geo_type = clean_text(source.get("network_type"))
        network_type = "warehouse" if geo_type in WAREHOUSE_GEO_TYPES else "regular"
        month1, month2, month3 = synthetic.choose("registry-months", name, MONTH_DISTRIBUTIONS)
        rows.append((
            name,
            kam,
            network_type,
            0 if unit_fraction(synthetic, "registry-active", name) < 0.05 else 1,
            month1, month2, month3,
            1 if unit_fraction(synthetic, "registry-cumulative", name) < 0.25 else 0,
            1 if unit_fraction(synthetic, "registry-vat", name) < 0.85 else 0,
            Decimal("20.00"),
            "sku" if unit_fraction(synthetic, "registry-entry-level", name) < 0.30 else "brand",
            "units" if unit_fraction(synthetic, "registry-entry-unit", name) < 0.25 else "rub",
        ))
    return rows


def insert_networks(cursor, rows: list[tuple[Any, ...]]) -> dict[str, int]:
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_Networks
            (name, kam, network_type, is_active, month1_pct, month2_pct, month3_pct,
             has_annual_investment_cumulative, vat_included, vat_rate,
             default_entry_level, default_entry_unit)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
        """,
        rows,
    )
    ids = fetch_dicts(cursor, "SELECT id, name FROM dbo.tbl_Networks")
    return {clean_text(row["name"]): int(row["id"]) for row in ids}


def insert_periods(
    cursor,
    network_ids: dict[str, int],
    profiles: dict[str, dict[str, Any]],
    years: Sequence[int],
) -> int:
    values = []
    for name, network_id in sorted(network_ids.items()):
        profile = profiles[name]
        for year in years:
            for quarter in (1, 2, 3, 4):
                values.append((network_id, year, quarter, profile["vat_included"], profile["vat_rate"]))
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_NetworkPeriods (network_id, [year], [quarter], vat_included, vat_rate)
        VALUES (?,?,?,?,?)
        """,
        values,
    )
    return len(values)


# ─── Факт отгрузок ─────────────────────────────────────────────────────────

def build_fact_rows(
    synthetic: StableSynthetic,
    facts: list[dict[str, Any]],
    network_ids: dict[str, int],
    sku_brands: dict[str, str],
    investment_pct: dict[str, Decimal],
) -> tuple[list[tuple[Any, ...]], dict[tuple[int, str, int, int], dict[str, Decimal]]]:
    """SKU-строки факта и квартальные итоги бренда для расчёта плана.

    Пишутся только строки по SKU: приложение само собирает из них итог бренда
    (aggregateFacts), а лишняя строка бренда дала бы вторую, расходящуюся сумму.
    """
    rows: list[tuple[Any, ...]] = []
    quarterly: dict[tuple[int, str, int, int], dict[str, Decimal]] = defaultdict(
        lambda: {"rub": Decimal(0), "units": Decimal(0)}
    )
    for source in facts:
        network = clean_text(source.get("networkName"))
        sku = clean_text(source.get("productName"))
        network_id = network_ids.get(network)
        brand = sku_brands.get(sku)
        if network_id is None or not brand:
            continue
        units = Decimal(str(source.get("units") or 0))
        rub = Decimal(str(source.get("rub") or 0))
        if units <= 0 or rub <= 0:
            continue
        year = int(source["year"])
        month = int(source["month"])

        pct = investment_pct.get(network, DEFAULT_INVESTMENT_PCT)
        jitter = pick_decimal(
            synthetic, "registry-fact-investments", f"{network}|{sku}|{year}|{month}",
            "0.85", "1.15",
        )
        investments = quantize_money(rub * pct / Decimal(100) * jitter)

        rows.append((
            network_id, year, month, brand, sku,
            quantize_money(rub), quantize_money(units), investments,
            1, "demo-shipments",
        ))
        bucket = quarterly[(network_id, brand, year, quarter_of(month))]
        bucket["rub"] += rub
        bucket["units"] += units
    return rows, quarterly


def insert_facts(cursor, rows: list[tuple[Any, ...]]) -> int:
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_NetworkMonthlyFacts
            (network_id, [year], [month], brand_as, sku,
             fact_rub, fact_units, fact_investments_rub, is_final, source_name)
        VALUES (?,?,?,?,?,?,?,?,?,?)
        """,
        rows,
        batch_size=500,
    )
    return len(rows)


# ─── Квартальные планы ─────────────────────────────────────────────────────

def build_plan_rows(
    synthetic: StableSynthetic,
    quarterly: dict[tuple[int, str, int, int], dict[str, Decimal]],
    network_ids: dict[str, int],
    profiles: dict[str, dict[str, Any]],
    investment_pct: dict[str, Decimal],
    fact_year: int,
    plan_years: Sequence[int],
) -> list[tuple[Any, ...]]:
    id_to_name = {value: key for key, value in network_ids.items()}
    known_fact_years = {key[2] for key in quarterly}
    rows: list[tuple[Any, ...]] = []
    gross_totals: dict[tuple[int, int, int], Decimal] = defaultdict(Decimal)

    for plan_year in plan_years:
        # Год со своим фактом опирается на него же, поэтому каждый закрытый год
        # показывает сравнение «план / факт». Год без факта берёт за основу
        # последний закрытый и растёт от него.
        base_year = plan_year if plan_year in known_fact_years else fact_year
        for (network_id, brand, year, quarter), bucket in sorted(
            quarterly.items(), key=lambda item: (item[0][0], item[0][1], item[0][2], item[0][3])
        ):
            if year != base_year:
                continue
            name = id_to_name.get(network_id)
            if name is None:
                continue
            profile = profiles[name]
            pct = investment_pct.get(name, DEFAULT_INVESTMENT_PCT)
            key = f"{name}|{brand}|{quarter}"
            price = bucket["rub"] / bucket["units"] if bucket["units"] > 0 else Decimal(0)
            is_gross = unit_fraction(synthetic, "registry-gross", name) < 0.10

            if plan_year == base_year:
                # План закрытого года восстанавливается из факта: часть кварталов
                # перевыполнена, часть провалена.
                achievement = pick_decimal(
                    synthetic, "registry-achievement", f"{key}|{plan_year}", *ACHIEVEMENT_BOUNDS,
                )
                plan_rub = bucket["rub"] / achievement
            else:
                growth = pick_decimal(
                    synthetic, "registry-growth", f"{key}|{plan_year}", *PLAN_GROWTH_BOUNDS,
                )
                plan_rub = bucket["rub"] * growth
            plan_rub = quantize_money(plan_rub)
            plan_units = quantize_money(plan_rub / price) if price > 0 else None

            # Уровень и единица ввода — свойство бренда в квартале: часть
            # брендов сети ведётся иначе, чем задано в карточке.
            entry_level = profile["default_entry_level"]
            if unit_fraction(synthetic, "registry-brand-level", key) < 0.25:
                entry_level = "sku" if entry_level == "brand" else "brand"
            entry_unit = profile["default_entry_unit"]
            if unit_fraction(synthetic, "registry-brand-unit", key) < 0.20:
                entry_unit = "units" if entry_unit == "rub" else "rub"

            rows.append((
                network_id, plan_year, quarter, brand, plan_rub, plan_units, pct,
                1 if is_gross else 0,
                profile["month1_pct"], profile["month2_pct"], profile["month3_pct"],
                entry_level, entry_unit, "demo-registry",
            ))
            if is_gross:
                gross_totals[(network_id, plan_year, quarter)] += plan_rub

    # Строка общего объёма валового контракта. Сама в пул не входит (in_gross=0)
    # и заведомо больше суммы брендов: в контракт входит не только они.
    for (network_id, plan_year, quarter), total in sorted(gross_totals.items()):
        name = id_to_name[network_id]
        profile = profiles[name]
        rows.append((
            network_id, plan_year, quarter, None,
            quantize_money(total * Decimal("1.08")), None,
            investment_pct.get(name, DEFAULT_INVESTMENT_PCT), 0,
            profile["month1_pct"], profile["month2_pct"], profile["month3_pct"],
            "brand", "rub", "demo-registry",
        ))
    return rows


def insert_plans(cursor, rows: list[tuple[Any, ...]]) -> int:
    execute_many(
        cursor,
        """
        INSERT INTO dbo.tbl_NetworkPlans
            (network_id, [year], [quarter], brand_as, plan_rub, plan_units, investments_pct,
             in_gross, month1_pct, month2_pct, month3_pct, entry_level, entry_unit, updated_by)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        """,
        rows,
        batch_size=500,
    )
    return len(rows)


def rollup_quarters(
    cursor,
    quarterly: dict[tuple[int, str, int, int], dict[str, Decimal]],
    plan_years: Sequence[int],
) -> int:
    """Переносит месячный факт в квартальные fact_rub и fact_investments_rub.

    Тем же запросом, что и загрузчик факта: демо пишет только строки по SKU, и
    ROLLUP_SQL сам берёт их сумму, потому что готового итога бренда (sku IS NULL)
    в demo-БД нет. Запускается после вставки планов — MERGE обновляет их строки.

    Сводятся только кварталы планируемых лет: у ветки WHEN NOT MATCHED свода нет
    ни плана, ни распределения по месяцам, и год без плана она завела бы строкой
    с одним фактом. Сейчас планы есть на оба года факта, так что отсев ничего не
    отбрасывает, — он держит функцию честной, если годы разойдутся снова.
    """
    years = set(plan_years)
    keys = sorted(key for key in quarterly if key[2] in years)
    execute_many(
        cursor,
        ROLLUP_SQL,
        [
            (network_id, year, (quarter - 1) * 3 + 1, quarter * 3, brand,
             network_id, year, quarter, brand)
            for network_id, brand, year, quarter in keys
        ],
        batch_size=500,
    )
    return len(keys)


# ─── Загрузка ──────────────────────────────────────────────────────────────

def registry_has_data(cursor) -> bool:
    return any(table_count(cursor, table) > 0 for table in REGISTRY_TABLES)


def clear_registry(cursor) -> None:
    for table in REGISTRY_TABLES:
        cursor.execute(f"DELETE FROM {table}")


def verify_target(cursor, fact_years: Sequence[int], plan_years: Sequence[int]) -> dict[str, Any]:
    cursor.execute(
        """
        SELECT COUNT_BIG(*),
               SUM(CASE WHEN kam IS NULL OR LTRIM(RTRIM(kam)) = '' THEN 1 ELSE 0 END),
               COUNT(DISTINCT kam),
               SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END),
               SUM(CASE WHEN name NOT LIKE N'Демо-сеть %' THEN 1 ELSE 0 END)
        FROM dbo.tbl_Networks
        """
    )
    row = cursor.fetchone()
    networks, without_kam, kams, active, foreign = (int(value or 0) for value in row)
    if without_kam:
        raise RuntimeError(f"У {without_kam} сетей реестра нет КАМа — фильтр по КАМ их не найдёт")
    if foreign:
        raise RuntimeError(f"В реестре {foreign} сетей вне демо-справочника")

    # КАМы реестра обязаны существовать в справочнике: иначе выбор в фильтре
    # даёт пустой список, хотя сеть у этого КАМа есть.
    cursor.execute(
        """
        SELECT COUNT_BIG(*) FROM dbo.tbl_Networks n
        WHERE n.kam IS NOT NULL AND NOT EXISTS (
            SELECT 1 FROM dbo.tbl_KAMNetworkMapping m WHERE m.kam = n.kam
        )
        """
    )
    unknown = int(cursor.fetchone()[0])
    if unknown:
        raise RuntimeError(f"{unknown} сетей реестра ссылаются на КАМа вне справочника")

    cursor.execute(
        """
        SELECT COUNT_BIG(*),
               SUM(CASE WHEN fact_rub < 0 OR fact_units < 0 OR fact_investments_rub < 0 THEN 1 ELSE 0 END),
               COUNT(DISTINCT network_id), COUNT(DISTINCT brand_as), COUNT(DISTINCT [year])
        FROM dbo.tbl_NetworkMonthlyFacts
        """
    )
    facts, negative, fact_networks, fact_brands, fact_year_count = (
        int(value or 0) for value in cursor.fetchone()
    )
    if negative:
        raise RuntimeError(f"В факте реестра {negative} отрицательных значений")

    cursor.execute(
        """
        SELECT [year], COUNT_BIG(*), COUNT(DISTINCT network_id)
        FROM dbo.tbl_NetworkPlans GROUP BY [year] ORDER BY [year]
        """
    )
    plans_by_year = {int(item[0]): int(item[1]) for item in cursor.fetchall()}
    for year in plan_years:
        if plans_by_year.get(year, 0) == 0:
            raise RuntimeError(f"Нет планов за {year} год")

    # Каждый закрытый год проверяется целиком: квартальный факт обязан быть
    # сведён, совпадать с помесячной таблицей и показывать обе стороны
    # выполнения — ровный перевес в одну означал бы нарисованную под ответ форму.
    plan_vs_fact: dict[str, dict[str, int]] = {}
    for year in plan_years:
        if year not in fact_years:
            continue

        # Вкладка «План и факт» читает квартальный факт из самой tbl_NetworkPlans,
        # поэтому без свода она показывает планы вообще без факта.
        cursor.execute(
            """
            SELECT COUNT_BIG(*), SUM(CASE WHEN fact_rub IS NULL THEN 1 ELSE 0 END)
            FROM dbo.tbl_NetworkPlans
            WHERE [year] = ? AND brand_as IS NOT NULL
            """,
            (year,),
        )
        brand_plans, without_fact = (int(value or 0) for value in cursor.fetchone())
        if brand_plans == 0:
            raise RuntimeError(f"Нет строк плана по брендам за {year} год")
        if without_fact:
            raise RuntimeError(
                f"У {without_fact} из {brand_plans} строк плана {year} года нет квартального факта"
            )

        # Свод обязан совпадать с помесячной таблицей: расхождение означало бы,
        # что квартал собран не из тех строк, которые показывает приложение.
        cursor.execute(
            """
            SELECT COUNT_BIG(*)
            FROM dbo.tbl_NetworkPlans p
            JOIN (
                SELECT network_id, brand_as, [year], (([month]-1)/3)+1 AS quarter,
                       SUM(fact_rub) AS fact_rub
                FROM dbo.tbl_NetworkMonthlyFacts
                GROUP BY network_id, brand_as, [year], (([month]-1)/3)+1
            ) f ON f.network_id = p.network_id AND f.brand_as = p.brand_as
               AND f.[year] = p.[year] AND f.quarter = p.[quarter]
            WHERE p.[year] = ? AND ABS(p.fact_rub - f.fact_rub) > 0.01
            """,
            (year,),
        )
        mismatched = int(cursor.fetchone()[0] or 0)
        if mismatched:
            raise RuntimeError(
                f"Квартальный факт {year} года расходится с помесячным в {mismatched} строках"
            )

        # Сравнивается та же колонка, которую читает приложение, а не заново
        # выведенный из помесячных строк квартал.
        cursor.execute(
            """
            SELECT SUM(CASE WHEN fact_rub > plan_rub THEN 1 ELSE 0 END),
                   SUM(CASE WHEN fact_rub <= plan_rub THEN 1 ELSE 0 END)
            FROM dbo.tbl_NetworkPlans
            WHERE [year] = ? AND plan_rub > 0 AND fact_rub IS NOT NULL
            """,
            (year,),
        )
        over, under = (int(value or 0) for value in cursor.fetchone())
        if over == 0 or under == 0:
            raise RuntimeError(
                f"План {year} года выполнен односторонне: перевыполнено {over}, провалено {under}"
            )
        plan_vs_fact[str(year)] = {"above": over, "below": under, "with_fact": brand_plans}

    return {
        "networks": networks,
        "active_networks": active,
        "distinct_kams": kams,
        "monthly_facts": facts,
        "fact_networks": fact_networks,
        "fact_brands": fact_brands,
        "fact_years": fact_year_count,
        "plans_by_year": {str(year): count for year, count in plans_by_year.items()},
        "plan_vs_fact": plan_vs_fact,
    }


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    env = dotenv_values(args.env_file)
    password = env.get("SA_PASSWORD") or os.getenv("SA_PASSWORD")
    target_db = args.target_db or env.get("DEMO_DB_NAME") or "local_project_demo_db"
    salt = env.get("DEMO_MAPPING_SALT") or env.get("JWT_SECRET") or os.getenv("DEMO_MAPPING_SALT")
    if not password:
        raise RuntimeError("SA_PASSWORD не задан")
    if not salt:
        raise RuntimeError("DEMO_MAPPING_SALT или JWT_SECRET не задан")
    if args.replace and args.confirm != RESET_CONFIRMATION:
        raise ValueError(f"Для --replace требуется --confirm {RESET_CONFIRMATION}")
    require_demo_target(target_db)

    synthetic = StableSynthetic(salt)
    target = connect(args.target_server, target_db, password, readonly=False)
    cursor = target.cursor()

    try:
        if registry_has_data(cursor) and not args.replace:
            raise RuntimeError(
                "Реестр demo-БД уже заполнен. Используйте --replace с защитной фразой."
            )

        networks = fetch_networks(cursor)
        sku_brands = fetch_sku_brands(cursor)
        if not networks or not sku_brands:
            raise RuntimeError(
                "В demo-БД нет промо-справочников; сначала выполните make demo-db-load"
            )
        _, ecom_last_year = fetch_ecom_years(cursor)
        fact_years = (ecom_last_year - 1, ecom_last_year)
        # Планы ведутся на оба года факта и на следующий: закрытые годы дают
        # сравнение с фактом, последний — работу с планом до отгрузок.
        plan_years = (*fact_years, ecom_last_year + 1)

        investment_pct = fetch_investment_pct(cursor)
        shipments = fetch_shipment_facts(cursor, fact_years[0], fact_years[1])
        if not shipments:
            raise RuntimeError("В интернет-продажах demo-БД нет сегмента OLAP NW")

        network_rows = build_network_rows(synthetic, networks)
        profiles = {
            row[0]: {
                "vat_included": row[8], "vat_rate": row[9],
                "month1_pct": row[4], "month2_pct": row[5], "month3_pct": row[6],
                "default_entry_level": row[10], "default_entry_unit": row[11],
            }
            for row in network_rows
        }

        if registry_has_data(cursor):
            clear_registry(cursor)

        network_ids = insert_networks(cursor, network_rows)
        periods = insert_periods(cursor, network_ids, profiles, plan_years)
        fact_rows, quarterly = build_fact_rows(
            synthetic, shipments, network_ids, sku_brands, investment_pct,
        )
        facts = insert_facts(cursor, fact_rows)
        target.commit()

        plan_rows = build_plan_rows(
            synthetic, quarterly, network_ids, profiles, investment_pct,
            ecom_last_year, plan_years,
        )
        plans = insert_plans(cursor, plan_rows)
        rolled_up = rollup_quarters(cursor, quarterly, plan_years)
        target.commit()

        try:
            verification = verify_target(cursor, fact_years, plan_years)
        except Exception:
            clear_registry(cursor)
            target.commit()
            raise
        target.commit()

        print(json.dumps({
            "status": "ok",
            "source_database_used": False,
            "target_database": target_db,
            "fact_years": list(fact_years),
            "plan_years": list(plan_years),
            "inserted": {
                "networks": len(network_rows),
                "periods": periods,
                "monthly_facts": facts,
                "plans": plans,
            },
            "rolled_up_quarters": rolled_up,
            "verification": verification,
        }, ensure_ascii=False, indent=2))
        return 0
    except Exception:
        target.rollback()
        raise
    finally:
        cursor.close()
        target.close()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"Ошибка наполнения реестра demo-БД: {error}", file=sys.stderr)
        raise SystemExit(1)
