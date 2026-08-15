# Analytics Portal

Аналитический портал для интернет-продаж и управления промо: Go/Gin API, React/Vite UI, Microsoft SQL Server и Python-скрипты синхронизации.

## Быстрый запуск через Docker

Требования: Docker Desktop и Docker Compose.

1. Создайте локальный файл окружения:

   ```bash
   cp .env.example .env
   ```

2. Замените `SA_PASSWORD`, `JWT_SECRET` и `BOOTSTRAP_PASSWORD` уникальными значениями. Реальные значения не должны попадать в Git.

3. Соберите и запустите весь стек:

   ```bash
   make up
   ```

   Backend дождётся SQL Server, создаст `DB_NAME`, если базы ещё нет, и применит все Goose-миграции.

4. Один раз создайте первого пользователя:

   ```bash
   make bootstrap-user
   ```

   Команда создаёт только нового пользователя. Существующий пароль или роль она не изменяет.

5. При необходимости добавьте демонстрационные данные (только не для `APP_ENV=production`):

   ```bash
   make seed-dev
   ```

После запуска:

- интерфейс: <http://localhost:5173>;
- API: <http://localhost:8080>;
- liveness: <http://localhost:8080/health>;
- readiness БД: <http://localhost:8080/ready>.

`make down` останавливает контейнеры, но сохраняет данные. Для полностью чистой локальной базы нужно явно удалить volume командой `docker compose down -v`; это необратимо для данных Docker-volume.

## Локальный запуск без Docker

Нужны Go 1.25+, Node.js 24+, SQL Server 2022 и ODBC Driver 17/18 для Python-скриптов.

Backend:

```bash
cp backend/.env.example backend/.env
cd backend
go run .
```

При `DB_AUTO_CREATE=true` пользователь SQL Server должен иметь право `CREATE DATABASE`. В production рекомендуется заранее создать базу отдельной административной учётной записью и установить `DB_AUTO_CREATE=false`.

Создание первого пользователя выполняется отдельно после применения миграций:

```bash
cd backend
go run ./cmd/bootstrap_user
```

Параметры берутся из `BOOTSTRAP_USERNAME`, `BOOTSTRAP_PASSWORD` и `BOOTSTRAP_ROLE` в окружении. Допустимые роли: `admin`, `agreement1`, `agreement2`.

Frontend:

```bash
cp frontend/.env.example frontend/.env
cd frontend
npm ci
npm run dev
```

## Схема базы

Goose-миграции встроены в backend-бинарник и применяются при запуске. Чистая установка создаёт:

- `tbl_PromoActivities`, `tbl_PromoComments`, `tbl_AuditLog`;
- `tbl_EcomSalesConsolidated`, `tbl_EcomSalesNormalized`;
- `tbl_ChannelSegmentMapping`, `tbl_SKUMapping`, `tbl_KAMNetworkMapping`, `tbl_MechanicsChannelMapping`, `tbl_NetworkGeoMapping`;
- `tbl_Users`.

Основные проверки месяцев, кварталов, ролей и статусов согласования закреплены ограничениями SQL Server. Для часто используемых фильтров и связей созданы индексы.

Dev-seed находится в `backend/cmd/seed_dev/dev.sql`, не содержит пользователей и повторно применяется безопасно.

## Синхронизация данных

```bash
cp sync_script/.env.example sync_script/.env
python3 -m venv sync_script/.venv
source sync_script/.venv/bin/activate
pip install -r sync_script/requirements.txt
python sync_script/sync_data.py
```

`sync_data.py` переносит интернет-продажи из OLAP и обновляет нормализованную таблицу.

Импорт промо выполняется через временную staging-таблицу и фиксируется одной транзакцией:

```bash
# Сначала проверить структуру, значения и дубли без подключения к БД
python sync_script/import_promo.py /path/to/promo.xlsx --dry-run

# Безопасный режим: добавить новые и обновить существующие записи
python sync_script/import_promo.py /path/to/promo.xlsx
```

По умолчанию записи, отсутствующие в Excel, не изменяются. Только если файл является полным снимком всего реестра, можно явно включить soft-delete отсутствующих активных записей:

```bash
python sync_script/import_promo.py /path/to/promo.xlsx --full-snapshot
```

Перед записью импорт отклоняет пустые и повторяющиеся бизнес-ключи, неверные числа и даты, несовместимую схему БД и уже существующие активные дубли. При любой ошибке вся транзакция откатывается.

## Проверки

Безопасный набор, не изменяющий рабочую БД:

```bash
make test
```

Полные интеграционные Go-тесты запускаются только с `DB_NAME`, оканчивающимся на `_test`; в противном случае они завершаются до подключения к базе.
