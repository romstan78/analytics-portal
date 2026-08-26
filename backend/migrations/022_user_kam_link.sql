-- +goose Up
-- Связь учётной записи с КАМом справочника.
--
-- Раньше её нигде не было: логин и имя КАМа в промо-строках жили отдельно,
-- поэтому определить «свои сети» для вошедшего КАМа было не по чему, и промо
-- показывались всем целиком. Колонка заполняется при заведении учётных
-- записей КАМов (sync_script/create_demo_kam_users.py).
--
-- NULL означает, что учётная запись ни за кем не закреплена, и видимость
-- промо для неё не ограничивается — так ведут себя администратор и
-- согласующие без области.
IF COL_LENGTH('dbo.tbl_Users', 'kam') IS NULL
    ALTER TABLE dbo.tbl_Users ADD kam NVARCHAR(255) NULL;

-- +goose Down
IF COL_LENGTH('dbo.tbl_Users', 'kam') IS NOT NULL
    ALTER TABLE dbo.tbl_Users DROP COLUMN kam;
