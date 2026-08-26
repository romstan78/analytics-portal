#!/usr/bin/env python3
"""Назначить пользователю область согласования: чьи промо он вправе видеть.

Паролей скрипт не касается — пользователь должен быть заведён заранее
(make demo-kam-users или make demo-bootstrap-user). Здесь только связка
«пользователь → ступень → КАМы».

    # демонстрационная связка: старший КАМ и четыре подчинённых
    python3 sync_script/set_demo_approval_scope.py --preset demo

    # произвольная связка
    python3 sync_script/set_demo_approval_scope.py \\
        --user kam.ershov.maksim --stage 2 --kam "Крылов Сергей" --kam "Жукова Ольга"

    # снять область: пользователь снова согласует всех, кого пускает роль
    python3 sync_script/set_demo_approval_scope.py --user kam.ershov.maksim --clear
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from dotenv import dotenv_values

from create_demo_promo_db import clean_text, connect, execute_many, fetch_dicts


# Демонстрационная связка. Старший КАМ ведёт свои сети и согласует на второй
# ступени промо четырёх подчинённых; собственные промо в область не входят и
# потому ему недоступны — для них достаточно первого согласующего.
DEMO_PRESET = {
    "user": "kam.ershov.maksim",
    "stage": 2,
    "kams": [
        "Алексеева Марина",
        "Крылов Сергей",
        "Жукова Ольга",
        "Данилова Елена",
    ],
}


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Назначить область согласования в demo-БД.")
    parser.add_argument("--env-file", type=Path, default=Path(".env"))
    parser.add_argument("--target-server", default="127.0.0.1,1434")
    parser.add_argument("--target-db", default=None)
    parser.add_argument("--preset", choices=["demo"], help="Готовая демонстрационная связка.")
    parser.add_argument("--user", help="Логин пользователя.")
    parser.add_argument("--stage", type=int, choices=[1, 2], help="Ступень согласования.")
    parser.add_argument("--kam", action="append", default=[], help="КАМ области; можно повторять.")
    parser.add_argument("--clear", action="store_true", help="Снять область целиком.")
    return parser.parse_args(argv)


def require_demo_target(target_db: str) -> None:
    if "demo" not in target_db.casefold():
        raise ValueError("Имя целевой базы обязано содержать 'demo'")


def resolve_request(args: argparse.Namespace) -> tuple[str, int, list[str]]:
    if args.preset == "demo":
        return DEMO_PRESET["user"], DEMO_PRESET["stage"], list(DEMO_PRESET["kams"])
    if not args.user:
        raise ValueError("Укажите --user или --preset demo")
    if args.clear:
        return args.user, 0, []
    if not args.stage:
        raise ValueError("Укажите --stage 1 или --stage 2")
    if not args.kam:
        raise ValueError("Укажите хотя бы один --kam")
    return args.user, args.stage, [clean_text(value) for value in args.kam]


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    env = dotenv_values(args.env_file)
    password = env.get("SA_PASSWORD") or os.getenv("SA_PASSWORD")
    target_db = args.target_db or env.get("DEMO_DB_NAME") or "local_project_demo_db"
    if not password:
        raise RuntimeError("SA_PASSWORD не задан")
    require_demo_target(target_db)

    username, stage, kams = resolve_request(args)
    connection = connect(args.target_server, target_db, password, readonly=False)
    cursor = connection.cursor()

    try:
        users = fetch_dicts(
            cursor, "SELECT username, role FROM dbo.tbl_Users WHERE username = ?", (username,),
        )
        if not users:
            raise RuntimeError(
                f"Пользователь {username!r} не заведён. Сначала выполните make demo-kam-users."
            )
        role = clean_text(users[0].get("role"))

        # КАМы области обязаны существовать в справочнике: иначе строка не
        # откроет ни одного промо и ошибку будет видно только на экране.
        known = {
            clean_text(row["kam"])
            for row in fetch_dicts(
                cursor,
                """
                SELECT DISTINCT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_PromoActivities
                WHERE deleted_at IS NULL AND kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
                """,
            )
        }
        unknown = [kam for kam in kams if kam not in known]
        if unknown:
            raise RuntimeError("КАМы вне справочника промо: " + ", ".join(unknown))

        cursor.execute("DELETE FROM dbo.tbl_ApprovalScope WHERE username = ?", (username,))
        if kams:
            execute_many(
                cursor,
                "INSERT INTO dbo.tbl_ApprovalScope(username, agreement_num, kam) VALUES (?,?,?)",
                [(username, stage, kam) for kam in kams],
            )
        connection.commit()

        visible = 0
        if kams:
            placeholders = ",".join("?" for _ in kams)
            cursor.execute(
                "SELECT COUNT(*) FROM dbo.tbl_PromoActivities "
                "WHERE deleted_at IS NULL AND kam IN (" + placeholders + ")",
                tuple(kams),
            )
            visible = int(cursor.fetchone()[0])

        print(json.dumps({
            "status": "ok",
            "target_database": target_db,
            "user": username,
            "role": role,
            "agreement_stage": stage if kams else None,
            "kams": kams,
            "promo_rows_in_scope": visible,
            "note": "пустой список означает согласование без ограничения по КАМам",
        }, ensure_ascii=False, indent=2))
        return 0
    except Exception:
        connection.rollback()
        raise
    finally:
        cursor.close()
        connection.close()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"Ошибка назначения области согласования: {error}", file=sys.stderr)
        raise SystemExit(1)
