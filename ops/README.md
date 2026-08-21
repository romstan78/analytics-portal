# Эксплуатация

## Production-конфигурация

1. Создайте базу и отдельного SQL-пользователя заранее.
2. Передайте секреты через secret manager или защищённое окружение.
3. Задайте публичные HTTPS-адреса `CORS_ORIGINS`, `PUBLIC_API_BASE` и точные IP/CIDR reverse proxy в `TRUSTED_PROXIES`.
4. Проверьте итоговую конфигурацию:

   ```bash
   docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
   ```

5. Запускайте стек с обоими compose-файлами. Production override запрещает автоматическое создание БД и не публикует порт SQL Server на хосте.

TLS должен завершаться на внешнем reverse proxy. Для production refresh-cookie помечается `Secure`.

## Проверка перед выпуском

```bash
make test
make test-e2e
make config-prod
```

CI описан в `.github/workflows/ci.yml` и состоит из трёх задач:

- `quality` — `go vet` и Go-тесты, тайпчек, линт, модульные тесты и сборка фронтенда, Python-тесты, проверка всех трёх сочетаний compose-файлов;
- `browser-smoke` — Playwright в Chromium;
- `clean-database` — поднимает чистый SQL Server, применяет все Goose-миграции и запускает интеграционные тесты; имя базы оканчивается на `_test`, иначе тесты сами откажутся стартовать.

Обновление workflow требует GitHub-токена со scope `workflow`.

## Резервное копирование

Compose подключает отдельный volume `mssql_backups` к `/var/opt/mssql/backup`. Перед миграцией или массовой операцией создайте `BACKUP DATABASE ... WITH COPY_ONLY, COMPRESSION, CHECKSUM`, затем обязательно выполните `RESTORE VERIFYONLY` для полученного `.bak`.

Проверяйте восстановление на отдельном SQL Server и под другим именем БД. Не запускайте `RESTORE ... WITH REPLACE` на рабочей базе. Срок хранения и внешнее копирование `.bak` должны настраиваться средствами инфраструктуры — Docker volume сам по себе не является внешней резервной копией.

## Наблюдаемость

- `/health` — процесс жив;
- `/ready` — SQL Server доступен;
- структурированные JSON-логи пишутся одновременно в stdout и ротируемый `backend_logs`;
- остановка по `SIGTERM` даёт активным запросам до 15 секунд на завершение.
