#!/usr/bin/env python3
"""Завести в demo-контуре по учётной записи на каждого КАМа справочника.

Пароли генерируются здесь же, на машине запускающего, и печатаются один раз в
его терминал — скрипт их никуда не сохраняет и никуда не отправляет. Хеширует
пароль не он, а штатная команда bootstrap-user внутри контейнера: там bcrypt с
тем же стоимостным параметром, проверка формата имени и отказ перезаписать уже
существующего пользователя.

    python3 sync_script/create_demo_kam_users.py --dry-run   # только имена
    python3 sync_script/create_demo_kam_users.py             # завести учётки
    python3 sync_script/create_demo_kam_users.py --link-only # только закрепление
"""

from __future__ import annotations

import argparse
import os
import re
import secrets
import string
import subprocess
import sys
from pathlib import Path

from dotenv import dotenv_values

from create_demo_promo_db import clean_text, connect, fetch_dicts


COMPOSE_FILE = "docker-compose.demo.yml"
BOOTSTRAP_SERVICE = "bootstrap-user"
KAM_ROLE = "kam"
PASSWORD_LENGTH = 16

# Практическая транслитерация: имена КАМов в справочнике русские, а
# BOOTSTRAP_USERNAME принимает только латиницу, цифры, точку, дефис и
# подчёркивание (usernamePattern в backend/cmd/bootstrap_user).
TRANSLIT = {
    "а": "a", "б": "b", "в": "v", "г": "g", "д": "d", "е": "e", "ё": "e",
    "ж": "zh", "з": "z", "и": "i", "й": "y", "к": "k", "л": "l", "м": "m",
    "н": "n", "о": "o", "п": "p", "р": "r", "с": "s", "т": "t", "у": "u",
    "ф": "f", "х": "kh", "ц": "ts", "ч": "ch", "ш": "sh", "щ": "shch",
    "ъ": "", "ы": "y", "ь": "", "э": "e", "ю": "yu", "я": "ya",
}


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Создать учётные записи КАМов демонстрационного контура."
    )
    parser.add_argument("--env-file", type=Path, default=Path(".env"))
    parser.add_argument("--target-server", default="127.0.0.1,1434")
    parser.add_argument("--target-db", default=None)
    parser.add_argument("--prefix", default="kam", help="Префикс логина (по умолчанию kam).")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Показать логины, которые будут заведены, не создавая их и не генерируя пароли.",
    )
    parser.add_argument(
        "--link-only",
        action="store_true",
        help="Только проставить закрепление за КАМом уже заведённым учётным "
             "записям. Пароли не генерируются и не печатаются.",
    )
    return parser.parse_args(argv)


def require_demo_target(target_db: str) -> None:
    if "demo" not in target_db.casefold():
        raise ValueError("Имя целевой базы обязано содержать 'demo'")


def transliterate(value: str) -> str:
    result = []
    for char in clean_text(value).casefold():
        if char in TRANSLIT:
            result.append(TRANSLIT[char])
        elif char.isascii() and (char.isalnum() or char in "._-"):
            result.append(char)
        elif char.isspace():
            result.append(".")
    return re.sub(r"\.+", ".", "".join(result)).strip(".")


def build_username(prefix: str, kam: str, taken: set[str]) -> str:
    base = f"{prefix}.{transliterate(kam)}" if prefix else transliterate(kam)
    base = base[:100].strip(".") or "kam"
    if len(base) < 3:
        base = f"{base}.kam"
    candidate = base
    suffix = 2
    while candidate in taken:
        candidate = f"{base[:96]}.{suffix}"
        suffix += 1
    taken.add(candidate)
    return candidate


def generate_password() -> str:
    """Пароль длиннее требуемых 12 символов, без похожих друг на друга знаков."""
    alphabet = (
        "".join(c for c in string.ascii_letters if c not in "lIO")
        + "".join(c for c in string.digits if c not in "01")
        + "!@#%*-_"
    )
    return "".join(secrets.choice(alphabet) for _ in range(PASSWORD_LENGTH))


def fetch_kams(cursor) -> list[str]:
    rows = fetch_dicts(
        cursor,
        """
        SELECT kam FROM (
            SELECT DISTINCT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_KAMNetworkMapping
            WHERE kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
            UNION
            SELECT DISTINCT LTRIM(RTRIM(kam)) AS kam FROM dbo.tbl_Networks
            WHERE kam IS NOT NULL AND LTRIM(RTRIM(kam)) <> ''
        ) options ORDER BY kam
        """,
    )
    return [clean_text(row["kam"]) for row in rows if clean_text(row.get("kam"))]


def fetch_existing_usernames(cursor) -> set[str]:
    rows = fetch_dicts(cursor, "SELECT username FROM dbo.tbl_Users")
    return {clean_text(row["username"]).casefold() for row in rows if row.get("username")}


def link_user_to_kam(cursor, username: str, kam: str) -> None:
    """Связать учётную запись с КАМом справочника.

    Без этой связи вошедшего КАМа не с чем сопоставить: логин и имя КАМа в
    промо живут отдельно, и «свои сети» определить не по чему. Связь
    проставляется и уже существующим пользователям, поэтому команду можно
    повторить, чтобы починить закрепление, не заводя учётку заново.
    """
    cursor.execute(
        "UPDATE dbo.tbl_Users SET kam = ?, updated_at = GETDATE() WHERE username = ?",
        (kam, username),
    )


def bootstrap_user(username: str, password: str, role: str) -> subprocess.CompletedProcess:
    """Завести пользователя штатной командой контура.

    Пароль уходит в окружение процесса docker compose, а не в аргументы
    команды: аргументы видны в списке процессов всей машине.
    """
    environment = dict(os.environ)
    environment["DEMO_BOOTSTRAP_USERNAME"] = username
    environment["DEMO_BOOTSTRAP_PASSWORD"] = password
    environment["DEMO_BOOTSTRAP_ROLE"] = role
    return subprocess.run(
        [
            "docker", "compose", "-f", COMPOSE_FILE, "--profile", "tools",
            "run", "--rm", "--no-TTY", BOOTSTRAP_SERVICE,
        ],
        env=environment,
        capture_output=True,
        text=True,
    )


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    env = dotenv_values(args.env_file)
    password = env.get("SA_PASSWORD") or os.getenv("SA_PASSWORD")
    target_db = args.target_db or env.get("DEMO_DB_NAME") or "local_project_demo_db"
    if not password:
        raise RuntimeError("SA_PASSWORD не задан")
    require_demo_target(target_db)

    connection = connect(args.target_server, target_db, password, readonly=False)
    cursor = connection.cursor()
    kams = fetch_kams(cursor)
    existing = fetch_existing_usernames(cursor)

    if not kams:
        raise RuntimeError("В demo-БД нет КАМов; сначала выполните make demo-db-load")

    taken: set[str] = set()
    planned = [(kam, build_username(args.prefix, kam, taken)) for kam in kams]

    if args.dry_run:
        print(f"КАМов в справочнике: {len(planned)}. Будут заведены логины:\n")
        for kam, username in planned:
            mark = " (уже есть — обновится только закрепление)" if username.casefold() in existing else ""
            print(f"  {kam:<24} → {username}{mark}")
        print("\nПароли на этом шаге не генерируются. Запустите без --dry-run.")
        cursor.close()
        connection.close()
        return 0

    if args.link_only:
        # Отдельный режим для починки закрепления: он ничего не заводит и
        # потому не печатает паролей.
        linked = 0
        for kam, username in planned:
            if username.casefold() not in existing:
                print(f"  {kam:<24} {username:<28} — не заведён, пропущен")
                continue
            link_user_to_kam(cursor, username, kam)
            linked += 1
            print(f"  {kam:<24} {username:<28} → закреплён")
        connection.commit()
        cursor.close()
        connection.close()
        print(f"\nЗакреплено за КАМом: {linked}.")
        return 0

    print(f"Завожу {len(planned)} учётных записей с ролью «{KAM_ROLE}».")
    print("Пароли показываются один раз — сохраните их сейчас.\n")
    print(f"{'КАМ':<24} {'Логин':<28} Пароль")
    print("-" * 74)

    created = skipped = failed = linked = 0
    for kam, username in planned:
        if username.casefold() in existing:
            link_user_to_kam(cursor, username, kam)
            linked += 1
            print(f"{kam:<24} {username:<28} — уже существует, закрепление обновлено")
            skipped += 1
            continue
        secret = generate_password()
        result = bootstrap_user(username, secret, KAM_ROLE)
        if result.returncode != 0:
            message = (result.stderr or result.stdout or "").strip().splitlines()
            print(f"{kam:<24} {username:<28} ОШИБКА: {message[-1] if message else 'см. вывод docker'}")
            failed += 1
            continue
        link_user_to_kam(cursor, username, kam)
        linked += 1
        print(f"{kam:<24} {username:<28} {secret}")
        created += 1

    print("-" * 74)
    connection.commit()
    print(f"Создано: {created}, пропущено: {skipped}, ошибок: {failed}, закреплено за КАМом: {linked}.")
    cursor.close()
    connection.close()
    if failed:
        return 1
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"Ошибка создания учётных записей КАМов: {error}", file=sys.stderr)
        raise SystemExit(1)
