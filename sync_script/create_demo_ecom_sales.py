#!/usr/bin/env python3
"""Сгенерировать синтетические интернет-продажи в изолированной demo-БД.

Рабочая БД здесь не участвует вообще: строки собираются из демо-справочников
самой demo-БД (сети, бренды, SKU) и из масштаба демо-промо. Поэтому исходные
объёмы, цены и контрагенты в демо-контур не попадают даже теоретически, а
экраны промо и интернет-продаж показывают сопоставимые величины.

Ряд каждой пары «сеть × SKU» — гладкая функция номера месяца: опорный объём ×
сезонность × тренд × медленная волна. Независимого шума по месяцам нет, поэтому
соседние месяцы отличаются на единицы процентов. Показатели «без Ecom»
получаются вычитанием онлайновой части из общей, а не отдельной генерацией, и
по построению не могут оказаться больше общей величины или уйти в минус.

Горизонт заканчивается полным календарным годом: дашборд сравнивает год к году
без выравнивания по месяцам, и на оборванном годе все показатели динамики стали
бы отрицательными из-за неполного периода, а не из-за самих данных.
"""

from __future__ import annotations

import argparse
import calendar
import csv
import json
import math
import os
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime
from decimal import Decimal, ROUND_DOWN, ROUND_HALF_UP
from pathlib import Path
from typing import Any, Iterable, Sequence

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


RESET_CONFIRMATION = "RESET_DEMO_ECOM_DB"

# Колонки-метрики широкой таблицы. Порядок совпадает с UNPIVOT в sync_data.py:
# набор обязан быть полным, иначе часть значений не попадёт в нормализованную
# таблицу, из которой читает приложение.
METRIC_COLUMNS = (
    "qty", "rub",
    "qty_ZC", "rub_ZC", "qty_PLZ", "rub_PLZ", "qty_AR", "rub_AR",
    "qty_OZ", "rub_OZ", "qty_EA", "rub_EA", "qty_PMP", "rub_PMP",
    "qty_OMNI", "rub_OMNI", "qty_NW", "rub_NW",
    "NW_wo_ecom", "SS_wo_ecom", "rub_NW_wo_ecom", "rub_SS_wo_ecom",
)

# Суффиксы колонок онлайн-площадок. Названия площадок и их разбивка по каналам
# берутся из справочника dimEcomSegment.csv и не меняются.
PLATFORM_CODES = ("ZC", "PLZ", "AR", "EA", "OZ", "PMP", "OMNI")

# Типы сетей, у которых онлайн — это практически весь оборот.
ONLINE_NETWORK_TYPES = ("E-Retailer", "Marketplace")

ECOM_TABLES = (
    "dbo.tbl_EcomSalesNormalized",
    "dbo.tbl_EcomSalesConsolidated",
    "dbo.tbl_ChannelSegmentMapping",
)

# Сезонность аптечного спроса: зимний пик, летний спад. Сумма — ровно 12, чтобы
# опорный объём читался как средний месяц года. Декабрь и январь близки, поэтому
# на стыке лет ряд не разрывается.
SEASONALITY = (
    Decimal("1.12"), Decimal("1.10"), Decimal("1.05"), Decimal("0.99"),
    Decimal("0.94"), Decimal("0.90"), Decimal("0.88"), Decimal("0.90"),
    Decimal("0.96"), Decimal("1.02"), Decimal("1.07"), Decimal("1.07"),
)

# Выход пары на полку: первые месяцы продаж заведомо меньше обычных, иначе
# появление новой пары читалось бы как ступенька совокупного объёма. Шаги
# подобраны так, чтобы сам разгон не давал скачка внутри ряда пары.
RAMP = (Decimal("0.78"), Decimal("0.85"), Decimal("0.91"), Decimal("0.96"))

# На сколько месяцев продажи пары начинаются раньше её первого промо.
PROMO_LEAD_MONTHS = 18

# Доля онлайна растёт линейно по абсолютному номеру месяца. Знаменатель —
# константа, а не длина горизонта: при повторном запуске с более поздней
# отсечкой ранее сгенерированные месяцы обязаны получиться прежними.
ECOM_SHARE_SPAN = 48

# Веса площадок внутри онлайна: первая площадка сети всегда крупнее остальных.
PLATFORM_WEIGHTS = (
    Decimal("0.42"), Decimal("0.24"), Decimal("0.15"),
    Decimal("0.10"), Decimal("0.05"), Decimal("0.03"), Decimal("0.01"),
)

# Годовой рост цены там, где в промо нет опорного значения за соседний год.
PRICE_YEAR_GROWTH = Decimal("1.06")

# Динамика не выдумывается, а берётся из демо-промо: оно получено из рабочей
# БД и уже несёт реальную картину — часть сетей и брендов теряет объём. Иначе
# спады, видимые в промо, пропали бы на экране продаж, и две витрины
# рассказывали бы про одни и те же сети разное.
#
# Берётся не средний тренд за горизонт, а погодовой индекс: дашборд сравнивает
# последний год с предыдущим, поэтому продажи должны повторять форму промо по
# годам, а не только общее направление.
#
# Промо-ряд копируется не один в один: он отражает промо-активность, и сеть,
# просто свернувшая акции, даёт там падение в десять раз. Индекс считается
# относительно рынка, сжимается корнем и ограничивается по шагу и по уровню:
# знак динамики сохраняется, а масштаб остаётся правдоподобным для полных
# продаж. Рынок в упаковках при этом держится ровно — рост выручки целиком
# объясняется ценой, а собственный рост промо-объёма сюда не годится, потому
# что он завышен ростом числа промо-строк год от года.
TREND_DAMPING = 0.5
LEVEL_STEP_BOUNDS = (Decimal("0.85"), Decimal("1.20"))
LEVEL_BOUNDS = (Decimal("0.45"), Decimal("2.20"))


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Создать синтетические интернет-продажи в изолированной demo-БД."
    )
    parser.add_argument("--env-file", type=Path, default=Path(".env"))
    parser.add_argument("--target-server", default="127.0.0.1,1434")
    parser.add_argument("--target-db", default=None)
    parser.add_argument("--segments-file", type=Path, default=None)
    parser.add_argument(
        "--last-period",
        default="",
        help="Последний месяц данных, YYYY-MM. По умолчанию — декабрь последнего "
             "полного года демо-промо.",
    )
    parser.add_argument("--replace", action="store_true")
    parser.add_argument("--confirm", default="")
    return parser.parse_args(argv)


def require_demo_target(target_db: str) -> None:
    if "demo" not in target_db.casefold():
        raise ValueError("Имя целевой базы обязано содержать 'demo'")


# ─── Детерминированные величины ────────────────────────────────────────────

def unit_fraction(synthetic: StableSynthetic, namespace: str, value: Any) -> float:
    """Число в [0,1) — устойчивая замена random() для одного значения ключа."""
    digest = synthetic.digest(namespace, value)
    return int.from_bytes(digest[:8], "big") / float(1 << 64)


def pick_decimal(
    synthetic: StableSynthetic,
    namespace: str,
    value: Any,
    low: str,
    high: str,
) -> Decimal:
    """Значение из отрезка [low, high] с шагом в сотые доли процента."""
    span = Decimal(high) - Decimal(low)
    return (Decimal(low) + span * Decimal(str(round(unit_fraction(synthetic, namespace, value), 6)))).quantize(
        Decimal("0.0001"), rounding=ROUND_HALF_UP
    )


def wave_factor(phases: tuple[float, float], index: int) -> Decimal:
    """Медленная волна ±5.5%: две синусоиды с периодами 17 и 29 месяцев.

    Периоды взаимно просты с 12, поэтому волна не складывается с сезонностью в
    один и тот же годовой узор, но при этом остаётся непрерывной по месяцам.
    """
    first = 0.035 * math.sin(2 * math.pi * (index / 17.0 + phases[0]))
    second = 0.020 * math.sin(2 * math.pi * (index / 29.0 + phases[1]))
    return Decimal(str(round(1.0 + first + second, 6)))


def split_units(total: int, weights: Sequence[Decimal]) -> list[int]:
    """Разложить целые упаковки по весам без потери и добавления единиц."""
    if not weights:
        return []
    if total <= 0:
        return [0] * len(weights)
    total_weight = sum(weights)
    raw = [Decimal(total) * weight / total_weight for weight in weights]
    parts = [int(value.to_integral_value(ROUND_DOWN)) for value in raw]
    remainder = total - sum(parts)
    order = sorted(range(len(raw)), key=lambda i: (raw[i] - parts[i], -i), reverse=True)
    for position in range(remainder):
        parts[order[position % len(order)]] += 1
    return parts


# ─── Периоды и цены ────────────────────────────────────────────────────────

def month_periods(first: tuple[int, int], last: tuple[int, int]) -> list[tuple[int, int]]:
    start = first[0] * 12 + (first[1] - 1)
    end = last[0] * 12 + (last[1] - 1)
    if end < start:
        raise ValueError("Последний месяц данных раньше первого")
    return [(value // 12, value % 12 + 1) for value in range(start, end + 1)]


def fill_price_anchors(
    anchors: dict[str, dict[int, Decimal]],
    skus: Iterable[str],
    years: Sequence[int],
    fallback: Decimal,
) -> dict[str, dict[int, Decimal]]:
    """Достроить опорные цены на годы, которых нет в промо.

    Пропуск заполняется ближайшим известным годом с годовым ростом, поэтому
    цена никогда не проваливается к нулю и не прыгает между годами.
    """
    filled: dict[str, dict[int, Decimal]] = {}
    for sku in skus:
        known = anchors.get(sku, {})
        if not known:
            known = {years[0]: fallback}
        series: dict[int, Decimal] = {}
        for year in years:
            if year in known:
                series[year] = known[year]
                continue
            nearest = min(known, key=lambda candidate: (abs(candidate - year), candidate))
            distance = year - nearest
            growth = PRICE_YEAR_GROWTH ** abs(distance)
            value = known[nearest] * growth if distance > 0 else known[nearest] / growth
            series[year] = value.quantize(Decimal("0.0001"), rounding=ROUND_HALF_UP)
        filled[sku] = series
    return filled


def interpolate_years(
    anchors: dict[int, Decimal],
    year: int,
    month: int,
    edge_growth: Decimal,
) -> Decimal:
    """Значение месяца — интерполяция между серединами соседних годов.

    Годовое значение относится к году целиком, поэтому оно ставится в середину
    года, а месяцы между серединами получают линейный переход. Разрыв между
    декабрём и январём при этом остаётся в пределах десятых процента: именно
    так годовые опоры превращаются в ряд без ступенек на стыке лет.
    """
    current = anchors[year]
    offset = (Decimal(month) - Decimal("6.5")) / Decimal(12)
    if offset >= 0:
        neighbour = anchors.get(year + 1, current * edge_growth)
    else:
        neighbour = anchors.get(year - 1, current / edge_growth)
    return current + (neighbour - current) * abs(offset)


def price_curve(anchors: dict[int, Decimal], year: int, month: int) -> Decimal:
    return interpolate_years(anchors, year, month, PRICE_YEAR_GROWTH)


def level_curve(anchors: dict[int, Decimal], year: int, month: int) -> Decimal:
    # За краями горизонта уровень держится ровно: продлевать тренд некуда.
    return interpolate_years(anchors, year, month, Decimal(1))


# ─── Профиль пары «сеть × SKU» ─────────────────────────────────────────────

@dataclass(frozen=True)
class PairProfile:
    network: str
    sku: str
    brand: str
    base_units: Decimal
    level_index: dict[int, Decimal]
    nw_ratio: Decimal
    nw_price_factor: Decimal
    offline_price_factor: Decimal
    platforms: tuple[str, ...]
    platform_weights: tuple[Decimal, ...]
    platform_price_factors: dict[str, Decimal]
    ecom_share_from: Decimal
    ecom_share_to: Decimal
    start_index: int
    wave_phases: tuple[float, float] = field(default=(0.0, 0.0))


def normalized_series(series: dict[int, Decimal], years: Sequence[int]) -> dict[int, Decimal]:
    """Ряд, продлённый на все годы горизонта и приведённый к первому году.

    Пропущенный год — это отсутствие промо, а не нулевые продажи, поэтому он
    закрывается последним известным уровнем, а не нулём.
    """
    filled: dict[int, Decimal] = {}
    carried: Decimal | None = None
    for year in years:
        value = series.get(year)
        if value is not None and value > 0:
            carried = value
        if carried is not None:
            filled[year] = carried
    first = next((filled[year] for year in years if year in filled), None)
    if first is None or first <= 0:
        return {year: Decimal(1) for year in years}
    return {year: filled.get(year, first) / first for year in years}


def trend_index(
    series: dict[int, Decimal],
    market: dict[int, Decimal],
    years: Sequence[int],
) -> dict[int, Decimal]:
    """Погодовой индекс уровня относительно рынка, сжатый и ограниченный.

    Первый год горизонта — единица. Дальше индекс идёт за промо: год, в котором
    сеть просела относительно рынка, опускает уровень продаж, и наоборот.
    """
    own = normalized_series(series, years)
    base = normalized_series(market, years)
    index: dict[int, Decimal] = {}
    level = Decimal(1)
    previous = Decimal(1)
    for position, year in enumerate(years):
        ratio = own[year] / base[year] if base[year] > 0 else Decimal(1)
        ratio = Decimal(str(round(float(ratio) ** TREND_DAMPING, 6)))
        if position > 0 and previous > 0:
            step = min(max(ratio / previous, LEVEL_STEP_BOUNDS[0]), LEVEL_STEP_BOUNDS[1])
            level = min(max(level * step, LEVEL_BOUNDS[0]), LEVEL_BOUNDS[1])
        previous = ratio
        index[year] = level
    return index


def combined_level_index(
    network_index: dict[int, Decimal],
    sku_index: dict[int, Decimal],
    years: Sequence[int],
) -> dict[int, Decimal]:
    combined: dict[int, Decimal] = {}
    for year in years:
        value = network_index.get(year, Decimal(1)) * sku_index.get(year, Decimal(1))
        combined[year] = min(max(value, LEVEL_BOUNDS[0]), LEVEL_BOUNDS[1])
    return combined


def network_platforms(
    synthetic: StableSynthetic,
    network: str,
    network_type: str,
) -> tuple[tuple[str, ...], tuple[Decimal, ...]]:
    """Набор площадок сети. Онлайн-игрок работает через одну-две площадки,
    аптечная сеть — через три-пять."""
    if network_type in ONLINE_NETWORK_TYPES:
        count = 1 + int(unit_fraction(synthetic, "platform-count-online", network) * 2)
    else:
        count = 3 + int(unit_fraction(synthetic, "platform-count", network) * 3)
    ordered = synthetic.order(f"platform-order:{network}", PLATFORM_CODES)
    chosen = tuple(ordered[:count])
    return chosen, tuple(PLATFORM_WEIGHTS[:count])


def build_pair_profiles(
    synthetic: StableSynthetic,
    promo_pairs: list[dict[str, Any]],
    networks: dict[str, dict[str, Any]],
    sku_brands: dict[str, str],
    first_year: int,
    years: Sequence[int],
    trends: dict[str, dict[str, dict[int, Decimal]]] | None = None,
) -> list[PairProfile]:
    network_trends = (trends or {}).get("network", {})
    sku_trends = (trends or {}).get("sku", {})
    profiles: list[PairProfile] = []
    for pair in sorted(promo_pairs, key=lambda row: (row["network_name"], row["sku"])):
        network = clean_text(pair.get("network_name"))
        sku = clean_text(pair.get("sku"))
        if not network or not sku or network not in networks or sku not in sku_brands:
            continue
        key = f"{network}|{sku}"
        network_type = clean_text(networks[network].get("network_type"))
        is_online = network_type in ONLINE_NETWORK_TYPES
        top20 = clean_text(networks[network].get("top20_segment")).casefold()

        # Опорный объём считается от пикового месяца промо, а не от годовой
        # суммы: иначе весь объём сети оказался бы промо-объёмом.
        peak = Decimal(str(pair.get("peak_units") or 0))
        floor = Decimal("420") if "top" in top20 else Decimal("160")
        multiplier = pick_decimal(synthetic, "ecom-scale", key, "1.7", "2.7")
        base_units = max(peak * multiplier, floor)

        # Продажи начинаются заметно раньше первого промо и в разные месяцы:
        # акцию ставят на товар, который уже продаётся. Полутора лет форы
        # хватает, чтобы вход пары не читался ни январской ступенькой, ни
        # мнимым ростом сети в тот год, когда она впервые попала в промо.
        first_promo_year = int(pair.get("first_year") or first_year)
        lead_months = PROMO_LEAD_MONTHS + int(unit_fraction(synthetic, "ecom-lead", key) * 9)
        start_index = max((first_promo_year - first_year) * 12 - lead_months, 0)

        if is_online:
            share_from = pick_decimal(synthetic, "ecom-share-from-online", network, "0.84", "0.91")
            share_to = pick_decimal(synthetic, "ecom-share-to-online", network, "0.93", "0.98")
        else:
            share_from = pick_decimal(synthetic, "ecom-share-from", network, "0.05", "0.09")
            share_to = pick_decimal(synthetic, "ecom-share-to", network, "0.15", "0.22")

        platforms, weights = network_platforms(synthetic, network, network_type)
        profiles.append(PairProfile(
            network=network,
            sku=sku,
            brand=sku_brands[sku],
            base_units=base_units,
            level_index=combined_level_index(
                network_trends.get(network, {}), sku_trends.get(sku, {}), years,
            ),
            nw_ratio=pick_decimal(synthetic, "ecom-nw-ratio", key, "0.94", "1.06"),
            nw_price_factor=pick_decimal(synthetic, "ecom-nw-price", sku, "0.78", "0.88"),
            offline_price_factor=pick_decimal(synthetic, "ecom-offline-price", sku, "0.99", "1.02"),
            platforms=platforms,
            platform_weights=weights,
            platform_price_factors={
                code: pick_decimal(synthetic, f"ecom-platform-price:{code}", sku, "0.94", "1.06")
                for code in platforms
            },
            ecom_share_from=share_from,
            ecom_share_to=share_to,
            start_index=start_index,
            wave_phases=(
                unit_fraction(synthetic, "ecom-wave-1", key),
                unit_fraction(synthetic, "ecom-wave-2", key),
            ),
        ))
    return profiles


def ecom_share(profile: PairProfile, index: int) -> Decimal:
    span = Decimal(ECOM_SHARE_SPAN)
    progress = min(Decimal(index) / span, Decimal(1))
    share = profile.ecom_share_from + (profile.ecom_share_to - profile.ecom_share_from) * progress
    return min(max(share, Decimal("0.02")), Decimal("0.98"))


def build_month_metrics(
    profile: PairProfile,
    index: int,
    month: int,
    price: Decimal,
    level: Decimal,
) -> dict[str, Decimal] | None:
    """Метрики одной строки широкой таблицы или None, если продаж ещё нет.

    level — уровень месяца относительно первого года горизонта; он приходит из
    погодового индекса промо, поэтому направление динамики совпадает с промо.
    """
    if index < profile.start_index:
        return None

    age = index - profile.start_index
    ramp = RAMP[age] if age < len(RAMP) else Decimal(1)
    units = (
        profile.base_units
        * SEASONALITY[month - 1]
        * level
        * wave_factor(profile.wave_phases, index)
        * ramp
    )
    total_units = max(int(units.to_integral_value(ROUND_HALF_UP)), 1)

    ecom_units = int((Decimal(total_units) * ecom_share(profile, index)).to_integral_value(ROUND_HALF_UP))
    # Офлайновая часть всегда остаётся положительной: «wo Ecom» вычитается из
    # общей величины, поэтому доля Ecom не может выйти за 100%.
    ecom_units = max(min(ecom_units, total_units - 1 if total_units > 1 else 0), 0)
    offline_units = total_units - ecom_units

    metrics: dict[str, Decimal] = {}
    offline_rub = quantize_money(Decimal(offline_units) * price * profile.offline_price_factor)
    ecom_rub = Decimal(0)
    for code, code_units in zip(profile.platforms, split_units(ecom_units, profile.platform_weights)):
        if code_units <= 0:
            continue
        code_rub = quantize_money(Decimal(code_units) * price * profile.platform_price_factors[code])
        metrics[f"qty_{code}"] = Decimal(code_units)
        metrics[f"rub_{code}"] = code_rub
        ecom_rub += code_rub

    nw_units = max(int((Decimal(total_units) * profile.nw_ratio).to_integral_value(ROUND_HALF_UP)), 1)
    nw_ecom_units = int((Decimal(ecom_units) * profile.nw_ratio).to_integral_value(ROUND_HALF_UP))
    nw_ecom_units = max(min(nw_ecom_units, nw_units - 1 if nw_units > 1 else 0), 0)
    nw_offline_units = nw_units - nw_ecom_units
    nw_price = price * profile.nw_price_factor
    nw_offline_rub = quantize_money(Decimal(nw_offline_units) * nw_price)
    nw_ecom_rub = quantize_money(Decimal(nw_ecom_units) * nw_price)

    # Итоги собираются сложением частей, а не отдельным округлением: иначе
    # копеечная разница нарушила бы равенство «всего = без Ecom + Ecom».
    metrics["qty"] = Decimal(total_units)
    metrics["rub"] = offline_rub + ecom_rub
    if offline_units > 0:
        metrics["SS_wo_ecom"] = Decimal(offline_units)
        metrics["rub_SS_wo_ecom"] = offline_rub
    metrics["qty_NW"] = Decimal(nw_units)
    metrics["rub_NW"] = nw_offline_rub + nw_ecom_rub
    if nw_offline_units > 0:
        metrics["NW_wo_ecom"] = Decimal(nw_offline_units)
        metrics["rub_NW_wo_ecom"] = nw_offline_rub
    return metrics


# ─── Справочник сегментов и каналов ────────────────────────────────────────

def resolve_segments_file(argument: Path | None) -> Path:
    if argument is not None:
        return argument
    candidates = (
        Path("dimEcomSegment.csv"),
        Path(__file__).resolve().parent.parent / "dimEcomSegment.csv",
    )
    for candidate in candidates:
        if candidate.exists():
            return candidate
    raise FileNotFoundError("Не найден справочник dimEcomSegment.csv; укажите --segments-file")


def read_segment_mapping(path: Path) -> list[tuple[str, str, str, str]]:
    """Прочитать dimEcomSegment.csv: имя метрики → единица, сегмент, канал.

    Названия площадок и каналов переносятся как есть: сегменты «OLAP SS»,
    «OLAP NW» и «… wo Ecom» захардкожены в backend и frontend, а разбивка по
    площадкам — сам справочник, ради которого строится демонстрация.
    """
    rows: list[tuple[str, str, str, str]] = []
    with path.open(encoding="utf-8-sig", newline="") as handle:
        reader = csv.reader(handle, delimiter=";")
        next(reader, None)
        for record in reader:
            if len(record) < 4:
                continue
            name = clean_text(record[0])
            if not name:
                continue
            rows.append((name, clean_text(record[1]), clean_text(record[2]), clean_text(record[3])))

    names = {row[0] for row in rows}
    missing = [column for column in METRIC_COLUMNS if column not in names]
    if missing:
        raise ValueError(f"В справочнике сегментов нет метрик: {', '.join(missing)}")
    unknown = sorted(names - set(METRIC_COLUMNS))
    if unknown:
        raise ValueError(f"В справочнике сегментов лишние метрики: {', '.join(unknown)}")
    return rows


# ─── Работа с demo-БД ──────────────────────────────────────────────────────

def fetch_networks(cursor) -> dict[str, dict[str, Any]]:
    rows = fetch_dicts(
        cursor,
        """
        SELECT network_name, network_type, top20_segment
        FROM dbo.tbl_NetworkGeoMapping
        ORDER BY network_name
        """,
    )
    return {clean_text(row["network_name"]): row for row in rows if row.get("network_name")}


def fetch_sku_brands(cursor) -> dict[str, str]:
    rows = fetch_dicts(cursor, "SELECT sku, brand, brand_as FROM dbo.tbl_SKUMapping ORDER BY sku")
    brands: dict[str, str] = {}
    for row in rows:
        sku = clean_text(row.get("sku"))
        if not sku:
            continue
        brands[sku] = clean_text(row.get("brand")) or clean_text(row.get("brand_as")) or "Демо-бренд"
    return brands


def fetch_promo_pairs(cursor) -> list[dict[str, Any]]:
    return fetch_dicts(
        cursor,
        """
        SELECT network_name, sku, MIN([year]) AS first_year, MAX(month_units) AS peak_units
        FROM (
            SELECT network_name, sku, [year], [month],
                   SUM(COALESCE(actual_promo_sales_units, plan_promo_units, 0)) AS month_units
            FROM dbo.tbl_PromoActivities
            WHERE deleted_at IS NULL AND network_name IS NOT NULL AND sku IS NOT NULL
            GROUP BY network_name, sku, [year], [month]
        ) monthly
        GROUP BY network_name, sku
        """,
    )


def fetch_promo_years(cursor) -> tuple[int, int]:
    """Первый год промо и последний год, закрытый всеми двенадцатью месяцами.

    Горизонт обрывается на полном годе намеренно: дашборд сравнивает год к году
    календарно, без выравнивания по месяцам (sales_service.go, dimensionValues).
    На неполном последнем году каждый драйвер и каждая строка рейтинга ушли бы
    в минус просто потому, что в текущем году месяцев меньше.
    """
    rows = fetch_dicts(
        cursor,
        """
        SELECT [year], COUNT(DISTINCT [month]) AS months
        FROM dbo.tbl_PromoActivities
        WHERE deleted_at IS NULL AND [year] IS NOT NULL AND [month] IS NOT NULL
        GROUP BY [year]
        """,
    )
    if not rows:
        raise RuntimeError("В demo-БД нет промо-периодов")
    years = [int(row["year"]) for row in rows]
    complete = [int(row["year"]) for row in rows if int(row["months"]) == 12]
    if not complete:
        raise RuntimeError("В demo-БД нет ни одного полного года промо")
    return min(years), max(complete)


def fetch_promo_trends(cursor, years: Sequence[int]) -> dict[str, dict[str, dict[int, Decimal]]]:
    """Погодовая динамика демо-промо по сетям и SKU, приведённая к рынку.

    Годы за пределами горизонта не берутся: неполный год занизил бы динамику
    всем сразу.
    """
    last_year = years[-1]
    def annual(column: str) -> dict[str, dict[int, Decimal]]:
        rows = fetch_dicts(
            cursor,
            f"""
            SELECT {column} AS name, [year],
                   SUM(COALESCE(actual_promo_sales_units, plan_promo_units, 0)) AS units
            FROM dbo.tbl_PromoActivities
            WHERE deleted_at IS NULL AND {column} IS NOT NULL AND [year] <= ?
            GROUP BY {column}, [year]
            """,
            (last_year,),
        )
        series: dict[str, dict[int, Decimal]] = {}
        for row in rows:
            name = clean_text(row.get("name"))
            if not name or row.get("year") is None:
                continue
            series.setdefault(name, {})[int(row["year"])] = Decimal(str(row.get("units") or 0))
        return series

    networks = annual("network_name")
    skus = annual("sku")

    market: dict[int, Decimal] = defaultdict(Decimal)
    for series in networks.values():
        for year, units in series.items():
            market[year] += units

    return {
        "network": {name: trend_index(series, market, years) for name, series in networks.items()},
        "sku": {name: trend_index(series, market, years) for name, series in skus.items()},
    }


def fetch_price_anchors(cursor) -> tuple[dict[str, dict[int, Decimal]], Decimal]:
    rows = fetch_dicts(
        cursor,
        """
        SELECT sku, [year], AVG(olap_price) AS price
        FROM dbo.tbl_PromoActivities
        WHERE deleted_at IS NULL AND olap_price > 0
        GROUP BY sku, [year]
        """,
    )
    anchors: dict[str, dict[int, Decimal]] = {}
    total = Decimal(0)
    count = 0
    for row in rows:
        sku = clean_text(row.get("sku"))
        if not sku or row.get("price") is None or row.get("year") is None:
            continue
        price = Decimal(str(row["price"])).quantize(Decimal("0.0001"), rounding=ROUND_HALF_UP)
        anchors.setdefault(sku, {})[int(row["year"])] = price
        total += price
        count += 1
    fallback = (total / count).quantize(Decimal("0.0001"), rounding=ROUND_HALF_UP) if count else Decimal("500")
    return anchors, fallback


def target_has_ecom_data(cursor) -> bool:
    return any(table_count(cursor, table) > 0 for table in ECOM_TABLES)


def clear_ecom_tables(cursor) -> None:
    for table in ECOM_TABLES:
        cursor.execute(f"DELETE FROM {table}")


def insert_segment_mapping(cursor, rows: list[tuple[str, str, str, str]]) -> int:
    execute_many(
        cursor,
        "INSERT INTO dbo.tbl_ChannelSegmentMapping(name,un_rub,segment,channel) VALUES (?,?,?,?)",
        rows,
    )
    return len(rows)


def insert_consolidated(cursor, rows: list[tuple[Any, ...]]) -> int:
    columns = ["[year]", "[month]", "brandName", "productName", "networkName"]
    columns += [f"[{column}]" for column in METRIC_COLUMNS]
    columns.append("updated_at")
    placeholders = ",".join("?" for _ in columns)
    execute_many(
        cursor,
        f"INSERT INTO dbo.tbl_EcomSalesConsolidated ({','.join(columns)}) VALUES ({placeholders})",
        rows,
        batch_size=500,
    )
    return len(rows)


# Нормализация повторяет sync_data.py: приложение читает только эту таблицу, а
# UNPIVOT сам отбрасывает NULL, поэтому непроданные площадки строк не создают.
NORMALIZE_QUERY = f"""
INSERT INTO dbo.tbl_EcomSalesNormalized
    (source_id,[year],[month],brandName,productName,networkName,
     metric_type,metric_value,updated_at)
SELECT id,[year],[month],brandName,productName,networkName,
       metric_type,metric_value,updated_at
FROM dbo.tbl_EcomSalesConsolidated
UNPIVOT (
    metric_value FOR metric_type IN ({','.join(METRIC_COLUMNS)})
) AS unpvt
WHERE metric_value <> 0 AND [year] = ?
"""

# Связка метрики со справочником делается отдельным UPDATE по колонкам таблиц:
# metric_type из UNPIVOT — системное имя колонки с собственным collation, и
# прямое сравнение с name справочника может упереться в конфликт сортировок.
MAP_SEGMENTS_QUERY = """
UPDATE n
SET n.un_rub = m.un_rub, n.segment = m.segment, n.channel = m.channel
FROM dbo.tbl_EcomSalesNormalized n
JOIN dbo.tbl_ChannelSegmentMapping m ON m.name = n.metric_type
WHERE n.[year] = ?
"""


def normalize_year(cursor, year: int) -> None:
    # Число вставленных строк здесь не снимается: соединение работает с
    # SET NOCOUNT ON, поэтому итог считает verify_target по самой таблице.
    cursor.execute(NORMALIZE_QUERY, (year,))
    cursor.execute(MAP_SEGMENTS_QUERY, (year,))


def verify_target(cursor, price_anchors: dict[str, dict[int, Decimal]]) -> dict[str, Any]:
    cursor.execute(
        """
        SELECT
          SUM(CASE WHEN COALESCE(SS_wo_ecom,0) > qty THEN 1 ELSE 0 END),
          SUM(CASE WHEN COALESCE(NW_wo_ecom,0) > qty_NW THEN 1 ELSE 0 END),
          SUM(CASE WHEN COALESCE(rub_SS_wo_ecom,0) > rub THEN 1 ELSE 0 END),
          SUM(CASE WHEN COALESCE(rub_NW_wo_ecom,0) > rub_NW THEN 1 ELSE 0 END),
          SUM(CASE WHEN ABS(rub - COALESCE(rub_SS_wo_ecom,0)
                    - COALESCE(rub_ZC,0) - COALESCE(rub_PLZ,0) - COALESCE(rub_AR,0)
                    - COALESCE(rub_EA,0) - COALESCE(rub_OZ,0) - COALESCE(rub_PMP,0)
                    - COALESCE(rub_OMNI,0)) > 0.01 THEN 1 ELSE 0 END),
          SUM(CASE WHEN qty - COALESCE(SS_wo_ecom,0)
                    - COALESCE(qty_ZC,0) - COALESCE(qty_PLZ,0) - COALESCE(qty_AR,0)
                    - COALESCE(qty_EA,0) - COALESCE(qty_OZ,0) - COALESCE(qty_PMP,0)
                    - COALESCE(qty_OMNI,0) <> 0 THEN 1 ELSE 0 END),
          COUNT_BIG(*)
        FROM dbo.tbl_EcomSalesConsolidated
        """
    )
    row = cursor.fetchone()
    broken = tuple(int(value or 0) for value in row[:6])
    if any(broken):
        raise RuntimeError(
            "Нарушены соотношения интернет-продаж demo-БД "
            f"(уп wo Ecom={broken[0]}, уп NW wo Ecom={broken[1]}, "
            f"руб wo Ecom={broken[2]}, руб NW wo Ecom={broken[3]}, "
            f"сумма руб={broken[4]}, сумма уп={broken[5]})"
        )
    consolidated_rows = int(row[6])

    cursor.execute(
        """
        SELECT COUNT_BIG(*),
               SUM(CASE WHEN metric_value < 0 THEN 1 ELSE 0 END),
               SUM(CASE WHEN segment IS NULL OR channel IS NULL OR un_rub IS NULL THEN 1 ELSE 0 END),
               SUM(CASE WHEN networkName NOT LIKE N'Демо-сеть %' THEN 1 ELSE 0 END),
               SUM(CASE WHEN productName NOT LIKE N'DB%-SKU%' THEN 1 ELSE 0 END),
               SUM(CASE WHEN brandName NOT LIKE N'Демо-бренд %' THEN 1 ELSE 0 END)
        FROM dbo.tbl_EcomSalesNormalized
        """
    )
    row = cursor.fetchone()
    normalized_rows = int(row[0])
    unsafe = tuple(int(value or 0) for value in row[1:])
    if any(unsafe):
        raise RuntimeError(
            "Проверка нормализованных интернет-продаж не пройдена "
            f"(отрицательные={unsafe[0]}, без справочника={unsafe[1]}, "
            f"сети={unsafe[2]}, SKU={unsafe[3]}, бренды={unsafe[4]})"
        )

    # Плавность: месяц к месяцу совокупный объём OLAP SS не должен прыгать.
    cursor.execute(
        """
        SELECT [year],[month],SUM(metric_value)
        FROM dbo.tbl_EcomSalesNormalized
        WHERE segment = N'OLAP SS' AND un_rub = N'уп'
        GROUP BY [year],[month]
        ORDER BY [year],[month]
        """
    )
    monthly = [(int(item[0]), int(item[1]), Decimal(str(item[2]))) for item in cursor.fetchall()]
    max_step = Decimal(0)
    max_step_period = None
    for previous, current in zip(monthly, monthly[1:]):
        if previous[2] == 0:
            continue
        step = abs(current[2] - previous[2]) / previous[2]
        if step > max_step:
            max_step = step
            max_step_period = f"{current[0]}-{current[1]:02d}"
    if max_step > Decimal("0.25"):
        raise RuntimeError(
            f"Скачок объёма OLAP SS в {max_step_period}: {max_step * 100:.1f}% к предыдущему месяцу"
        )

    # Доля Ecom по годам: она обязана быть положительной и меньше 100%.
    cursor.execute(
        """
        SELECT [year],
               SUM(CASE WHEN segment = N'OLAP SS' THEN metric_value ELSE 0 END),
               SUM(CASE WHEN segment = N'OLAP SS wo Ecom' THEN metric_value ELSE 0 END)
        FROM dbo.tbl_EcomSalesNormalized
        WHERE un_rub = N'уп' AND segment IN (N'OLAP SS', N'OLAP SS wo Ecom')
        GROUP BY [year] ORDER BY [year]
        """
    )
    ecom_share_by_year: dict[str, float] = {}
    for item in cursor.fetchall():
        full = Decimal(str(item[1]))
        without = Decimal(str(item[2]))
        if full <= 0:
            raise RuntimeError(f"Нулевой объём OLAP SS за {item[0]} год")
        share = (full - without) / full
        if not (Decimal(0) < share < Decimal(1)):
            raise RuntimeError(f"Недопустимая доля Ecom за {item[0]} год: {share}")
        ecom_share_by_year[str(int(item[0]))] = float(round(share * 100, 2))

    # Цена OLAP SS — то, что реестр сетей подставит как цену контракта. Она
    # должна остаться рядом с olap_price демо-промо, иначе экраны разойдутся.
    cursor.execute(
        """
        SELECT productName, [year],
               SUM(CASE WHEN un_rub = N'руб' THEN metric_value ELSE 0 END)
                 / NULLIF(SUM(CASE WHEN un_rub = N'уп' THEN metric_value ELSE 0 END), 0)
        FROM dbo.tbl_EcomSalesNormalized
        WHERE segment = N'OLAP SS'
        GROUP BY productName, [year]
        """
    )
    max_deviation = Decimal(0)
    worst = None
    for item in cursor.fetchall():
        sku = clean_text(item[0])
        year = int(item[1])
        if item[2] is None:
            continue
        anchor = price_anchors.get(sku, {}).get(year)
        if anchor is None or anchor == 0:
            continue
        deviation = abs(Decimal(str(item[2])) - anchor) / anchor
        if deviation > max_deviation:
            max_deviation = deviation
            worst = f"{sku} {year}"
    if max_deviation > Decimal("0.15"):
        raise RuntimeError(
            f"Цена OLAP SS расходится с промо по {worst}: {max_deviation * 100:.1f}%"
        )

    return {
        "consolidated_rows": consolidated_rows,
        "normalized_rows": normalized_rows,
        "max_month_over_month_step_pct": float(round(max_step * 100, 2)),
        "ecom_share_by_year_pct": ecom_share_by_year,
        "max_price_deviation_from_promo_pct": float(round(max_deviation * 100, 2)),
    }


def parse_period(value: str) -> tuple[int, int]:
    parts = value.split("-")
    if len(parts) != 2:
        raise ValueError("Ожидается период в формате YYYY-MM")
    year, month = int(parts[0]), int(parts[1])
    if not 1 <= month <= 12:
        raise ValueError("Месяц периода вне диапазона 1..12")
    return year, month


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

    segment_rows = read_segment_mapping(resolve_segments_file(args.segments_file))
    last_period = parse_period(args.last_period) if args.last_period else None

    synthetic = StableSynthetic(salt)
    target = connect(args.target_server, target_db, password, readonly=False)
    cursor = target.cursor()

    try:
        # Занятость целевых таблиц проверяется до генерации: собирать сотни
        # тысяч строк ради отказа на последнем шаге незачем.
        if target_has_ecom_data(cursor) and not args.replace:
            raise RuntimeError(
                "Demo-БД уже содержит интернет-продажи. Используйте --replace с защитной фразой."
            )

        networks = fetch_networks(cursor)
        sku_brands = fetch_sku_brands(cursor)
        promo_pairs = fetch_promo_pairs(cursor)
        if not networks or not sku_brands or not promo_pairs:
            raise RuntimeError(
                "В demo-БД нет промо-справочников; сначала выполните make demo-db-load"
            )
        anchors, fallback_price = fetch_price_anchors(cursor)
        first_year, last_full_year = fetch_promo_years(cursor)

        periods = month_periods((first_year, 1), last_period or (last_full_year, 12))
        years = sorted({year for year, _ in periods})
        price_anchors = fill_price_anchors(anchors, sku_brands, years, fallback_price)
        trends = fetch_promo_trends(cursor, years)
        profiles = build_pair_profiles(
            synthetic, promo_pairs, networks, sku_brands, first_year, years, trends,
        )

        rows: list[tuple[Any, ...]] = []
        for index, (year, month) in enumerate(periods):
            last_day = calendar.monthrange(year, month)[1]
            updated_at = datetime(year, month, last_day, 4, 15)
            for profile in profiles:
                price = price_curve(price_anchors[profile.sku], year, month)
                level = level_curve(profile.level_index, year, month)
                metrics = build_month_metrics(profile, index, month, price, level)
                if metrics is None:
                    continue
                rows.append((
                    year, month, profile.brand, profile.sku, profile.network,
                    *(metrics.get(column) for column in METRIC_COLUMNS),
                    updated_at,
                ))

        if target_has_ecom_data(cursor):
            clear_ecom_tables(cursor)

        mapping_rows = insert_segment_mapping(cursor, segment_rows)
        insert_consolidated(cursor, rows)
        target.commit()

        for year in years:
            normalize_year(cursor, year)
            target.commit()

        # Строки нормализуются и фиксируются по годам, чтобы не держать всю
        # выгрузку в одной транзакции, поэтому откатить непрошедшую проверку
        # уже нельзя — вместо отката таблицы очищаются. Оставлять в demo-БД
        # непроверенные продажи хуже, чем оставить её пустой.
        try:
            verification = verify_target(cursor, price_anchors)
        except Exception:
            clear_ecom_tables(cursor)
            target.commit()
            raise
        target.commit()

        print(json.dumps({
            "status": "ok",
            "source_database_used": False,
            "target_database": target_db,
            "period": {
                "from": f"{periods[0][0]}-{periods[0][1]:02d}",
                "to": f"{periods[-1][0]}-{periods[-1][1]:02d}",
                "months": len(periods),
            },
            "references": {
                "channel_segment_mapping": mapping_rows,
                "networks": len({profile.network for profile in profiles}),
                "skus": len({profile.sku for profile in profiles}),
                "network_sku_pairs": len(profiles),
            },
            "inserted": {"consolidated_rows": len(rows)},
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
        print(f"Ошибка создания интернет-продаж demo-БД: {error}", file=sys.stderr)
        raise SystemExit(1)
