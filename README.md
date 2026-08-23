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
- `tbl_Networks`, `tbl_NetworkPeriods`, `tbl_NetworkPlans`, `tbl_NetworkComments`;
- `tbl_Users`, `tbl_RefreshSessions`.

Основные проверки месяцев, кварталов, ролей и статусов согласования закреплены ограничениями SQL Server. Для часто используемых фильтров и связей созданы индексы.

Dev-seed находится в `backend/cmd/seed_dev/dev.sql`, не содержит пользователей и повторно применяется безопасно.

## Выгрузка интернет-продаж

`GET /api/data?all=true` отдаёт выборку целиком, построчно, не собирая её в памяти сервера. Размер ограничен переменной `SALES_EXPORT_MAX_ROWS` (по умолчанию 200000): при превышении запрос отклоняется с кодом `413` и текстом, сколько строк запрошено и каков лимит. Для больших объёмов используйте `GET /api/data/export-xlsx` — этот эндпоинт формирует файл потоком и лимитом не ограничен.

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

### Помесячный факт реестра сетей

План КАМ задаёт по кварталам, прогноз — по месяцам, а факт приходит выгрузкой отгрузок
и в интерфейсе доступен только для чтения:

```bash
# Проверить файл и сопоставление сетей, ничего не записывая
python sync_script/import_network_facts.py /path/to/facts.xlsx --dry-run

# Загрузить
python sync_script/import_network_facts.py /path/to/facts.xlsx
```

Ожидаемые колонки (Excel или CSV; регистр и пунктуация в заголовках не важны):

| Колонка | Обязательна | Комментарий |
| --- | --- | --- |
| `Сеть` | да | сопоставляется с названием в реестре |
| `Бренд` | да | `brand_as` строки плана |
| `SKU` | нет | если пусто, строка считается итогом бренда |
| `Год`, `Месяц` | да | месяц 1–12 |
| `Факт, руб` | одна из трёх | фактический объём |
| `Факт, уп` | одна из трёх | фактические упаковки |
| `Факт инвестиций, руб` | одна из трёх | фактические инвестиции |

Файл проверяется целиком до записи: пустые ключи, нечисловые и отрицательные значения, неверный месяц и дубли строк перечисляются одним списком. Загрузка и квартальный roll-up выполняются одной транзакцией. Сети, которых нет в реестре, останавливают загрузку и выводятся с подсказкой похожих названий:

```
Не найдено сетей в реестре: 2
  «Магнит ММ» — похоже на: Магнит
  «Ашан» — похожих в реестре нет
```

Их либо переименовывают в файле, либо заводят в реестре, либо пропускают явным флагом `--allow-unknown-networks`.

Пустая ячейка означает «значение не пришло» и уже загруженное не затирает, поэтому объём и инвестиции можно грузить разными файлами. Бренд, которого нет в плане, добавляется новой строкой вне валового объёма: факт по незапланированному бренду не теряется.

### Импорт промо

Экспорт портала содержит колонку `ID промо`. Для существующих записей импорт использует этот ID и поэтому корректно различает несколько промо с одинаковыми сетью, SKU, месяцем и механикой. Для строк без ID старый составной ключ используется только при однозначном совпадении; неоднозначный импорт блокируется. Поля согласования управляются приложением и из Excel не перезаписываются.

Перед записью импорт отклоняет пустые ключи, повторяющиеся ID, неверные числа и даты, несовместимую схему БД и неоднозначные совпадения. При любой ошибке вся транзакция откатывается.

### Аудит и очистка точных дублей промо

Сначала сформируйте JSON-план только чтением:

```bash
python sync_script/dedupe_promo.py report --output /secure/path/promo-dedup-plan.json
```

План разделяет группы на `exact_duplicate`, `approval_conflict` и `data_conflict`. Автоматически применяются только полностью идентичные записи; конфликтные промо не изменяются.

```bash
python sync_script/dedupe_promo.py apply \
  --plan /secure/path/promo-dedup-plan.json \
  --confirm APPLY_SAFE_EXACT_DUPLICATES
```

Комментарии и аудит переносятся на выбранную основную запись, а лишние строки только помечаются удалёнными. Все перемещения сохраняются в таблицах журнала миграции `007`, поэтому запуск можно полностью откатить по напечатанному `run_id`:

```bash
python sync_script/dedupe_promo.py rollback \
  --run-id 00000000-0000-0000-0000-000000000000 \
  --confirm ROLLBACK_SAFE_DEDUP
```

## Проверки

Безопасный набор, не изменяющий рабочую БД:

```bash
make test
```

Браузерные smoke-тесты запускаются отдельно:

```bash
make test-e2e
```

Полные интеграционные Go-тесты запускаются только с `DB_NAME`, оканчивающимся на `_test`; в противном случае они завершаются до подключения к базе.

## Production

Production override отключает автоматическое создание БД, требует явные HTTPS-адреса и скрывает порт SQL Server от хоста:

```bash
docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
docker compose -f docker-compose.yml -f docker-compose.production.yml up -d --build
```

Пример несекретных параметров находится в `ops/production.env.example`, эксплуатационный чек-лист — в `ops/README.md`. Пароли и JWT-секрет передаются только через защищённое окружение или secret manager.
