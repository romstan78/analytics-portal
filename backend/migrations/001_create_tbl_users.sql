-- +goose Up
-- Migration: создание tbl_Users и seed пользователей.
-- Пароли захешированы bcrypt (cost=10):
--   manager1 / promo2024!   → $2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2
--   manager2 / promo2024!   → $2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2
--   admin    / admin2024!   → $2a$10$jyr5S3OrcUK5UmgUwsnDCeUjhEHdcQji7tO.L.y0oAncVBA/YTzFO

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

-- Seed тестовых пользователей (идемпотентно).
IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager1' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager1', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement1');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager2' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager2', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement2');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'admin' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('admin', '$2a$10$jyr5S3OrcUK5UmgUwsnDCeUjhEHdcQji7tO.L.y0oAncVBA/YTzFO', 'admin');

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_Users;