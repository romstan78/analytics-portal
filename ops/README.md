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

## Сессии и отзыв доступа

Refresh-токены зарегистрированы в `dbo.tbl_RefreshSessions`; хранится только SHA-256 от токена, поэтому содержимое таблицы нельзя предъявить как токен. Токен одноразовый: при обновлении старая сессия гасится и выдаётся новая.

- выход (`POST /api/auth/logout`) отзывает сессию на сервере — перехваченный ранее токен становится бесполезен;
- повторное предъявление уже использованного токена трактуется как компрометация: гасятся все сессии пользователя, в журнале появляется `refresh_token_reuse_detected`;
- отозвать доступ вручную можно, проставив `revoked_at` нужным строкам:

```sql
UPDATE dbo.tbl_RefreshSessions
SET revoked_at = SYSUTCDATETIME(), revoke_cause = 'user_revoked'
WHERE username = 'ivanov' AND revoked_at IS NULL;
```

Отзыв действует на обновление сессии; уже выданный access-токен живёт до истечения своих 15 минут. Просроченные записи удаляются при входе пользователей.

После выкатки этого изменения ранее выданные refresh-cookie перестают работать: пользователям потребуется войти заново один раз.

## Проверка перед выпуском

```bash
make test
make test-e2e
make config-prod
```

Шаблон `ops/github-actions-ci.yml.example` дополнительно поднимает чистый SQL Server, применяет все Goose-миграции и запускает интеграционные тесты только на БД с суффиксом `_test`. Для активации скопируйте его в `.github/workflows/ci.yml` с GitHub-токеном, имеющим scope `workflow`.

## Резервное копирование

Compose подключает отдельный volume `mssql_backups` к `/var/opt/mssql/backup`. Перед миграцией или массовой операцией создайте `BACKUP DATABASE ... WITH COPY_ONLY, COMPRESSION, CHECKSUM`, затем обязательно выполните `RESTORE VERIFYONLY` для полученного `.bak`.

Проверяйте восстановление на отдельном SQL Server и под другим именем БД. Не запускайте `RESTORE ... WITH REPLACE` на рабочей базе. Срок хранения и внешнее копирование `.bak` должны настраиваться средствами инфраструктуры — Docker volume сам по себе не является внешней резервной копией.

## Наблюдаемость

- `/health` — процесс жив;
- `/ready` — SQL Server доступен;
- структурированные JSON-логи пишутся одновременно в stdout и ротируемый `backend_logs`;
- остановка по `SIGTERM` даёт активным запросам до 15 секунд на завершение.
