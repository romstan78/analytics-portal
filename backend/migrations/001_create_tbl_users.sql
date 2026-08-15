-- +goose Up
-- Migration: создание tbl_Users.
-- Пользователи не создаются основной миграцией: production-администратор
-- должен быть добавлен отдельной bootstrap-командой с уникальным паролем.

IF OBJECT_ID('dbo.tbl_Users', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_Users (
        id          INT IDENTITY PRIMARY KEY,
        username    NVARCHAR(100) NOT NULL UNIQUE,
        password_hash NVARCHAR(255) NOT NULL,
        role        NVARCHAR(50) NOT NULL DEFAULT 'agreement1',
        created_at  DATETIME DEFAULT GETDATE(),
        updated_at  DATETIME DEFAULT GETDATE(),
        deleted_at  DATETIME NULL
    )
END;

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_Users;
