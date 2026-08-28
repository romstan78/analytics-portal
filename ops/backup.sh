#!/usr/bin/env bash
#
# Резервная копия базы с проверкой и ротацией.
#
# Раньше ops/README.md описывал правильную ручную процедуру, но выполнял её
# человек: расписания не было, старые .bak никто не удалял, а копия оставалась
# в том же Docker volume, что и сама база. Этот скрипт делает первые две вещи;
# третью — вынос копии за пределы хоста — обязан делать вызывающий, потому что
# приёмник зависит от инфраструктуры (см. BACKUP_REMOTE_CMD).
#
# Запуск (пример для cron, ежедневно в 02:30):
#   30 2 * * * /path/to/ops/backup.sh >> /var/log/mssql-backup.log 2>&1
#
# Переменные:
#   DB_NAME              база (по умолчанию из .env)
#   SA_PASSWORD          пароль sa (по умолчанию из .env)
#   BACKUP_CONTAINER     имя контейнера SQL Server (по умолчанию mssql_db)
#   BACKUP_DIR           каталог внутри контейнера (по умолчанию /var/opt/mssql/backup)
#   BACKUP_KEEP_DAYS     сколько дней хранить копии (по умолчанию 14)
#   BACKUP_REMOTE_CMD    команда выноса копии наружу; получает путь файлом $1

set -euo pipefail

cd "$(dirname "$0")/.."

# .env читается построчно, а не через source: значения там не экранированы
# (пароль и JWT-секрет содержат пробелы и угловые скобки), и shell попытался бы
# выполнить их как перенаправление.
read_env() {
    local key="$1" line value
    [[ -f .env ]] || return 0
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ "$line" == "${key}="* ]] || continue
        value="${line#"${key}"=}"
        # Снимаем обрамляющие кавычки, если они есть.
        [[ "$value" == \"*\" ]] && value="${value:1:${#value}-2}"
        [[ "$value" == \'*\' ]] && value="${value:1:${#value}-2}"
        printf '%s' "$value"
        return 0
    done < .env
}

DB_NAME="${DB_NAME:-$(read_env DB_NAME)}"
SA_PASSWORD="${SA_PASSWORD:-$(read_env SA_PASSWORD)}"
: "${DB_NAME:?укажите DB_NAME в .env или окружении}"
: "${SA_PASSWORD:?укажите SA_PASSWORD в .env или окружении}"
BACKUP_CONTAINER="${BACKUP_CONTAINER:-mssql_db}"
BACKUP_DIR="${BACKUP_DIR:-/var/opt/mssql/backup}"
BACKUP_KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"

STAMP="$(date +%Y%m%d-%H%M%S)"
FILE="${BACKUP_DIR}/${DB_NAME}-${STAMP}.bak"

sql() {
    docker exec -i "$BACKUP_CONTAINER" /opt/mssql-tools18/bin/sqlcmd \
        -S localhost -U sa -P "$SA_PASSWORD" -C -b -Q "$1"
}

# Каталог тома создаётся Docker от root, а SQL Server работает от mssql и
# записать в него не может: BACKUP падает с «Operating system error 5». Права
# выставляются здесь, потому что том пересоздаётся вместе со стендом.
docker exec -u 0 "$BACKUP_CONTAINER" sh -c \
    "mkdir -p '${BACKUP_DIR}' && chown mssql:mssql '${BACKUP_DIR}'"

echo "[$(date -Iseconds)] бэкап ${DB_NAME} → ${FILE}"

# COPY_ONLY не сбивает цепочку differential-копий, CHECKSUM ловит битые страницы
# на записи, а не при восстановлении — когда копия уже нужна.
sql "BACKUP DATABASE [${DB_NAME}] TO DISK = N'${FILE}'
     WITH COPY_ONLY, COMPRESSION, CHECKSUM, INIT, STATS = 25;"

# Непроверенная копия — не копия: RESTORE VERIFYONLY читает файл целиком и
# сверяет контрольные суммы.
echo "[$(date -Iseconds)] проверка копии"
sql "RESTORE VERIFYONLY FROM DISK = N'${FILE}' WITH CHECKSUM;"

# Ротация: старые копии удаляются только после успешной проверки свежей.
echo "[$(date -Iseconds)] ротация старше ${BACKUP_KEEP_DAYS} дней"
docker exec "$BACKUP_CONTAINER" find "$BACKUP_DIR" \
    -maxdepth 1 -name "${DB_NAME}-*.bak" -type f -mtime "+${BACKUP_KEEP_DAYS}" -delete

# Вынос за пределы хоста. Docker volume рядом с базой резервной копией не
# является: он умирает вместе с хостом.
if [[ -n "${BACKUP_REMOTE_CMD:-}" ]]; then
    echo "[$(date -Iseconds)] вынос копии: ${BACKUP_REMOTE_CMD}"
    "${BACKUP_REMOTE_CMD}" "$FILE"
else
    echo "[$(date -Iseconds)] ВНИМАНИЕ: BACKUP_REMOTE_CMD не задан — копия осталась" \
         "на том же хосте, что и база"
fi

echo "[$(date -Iseconds)] готово"
