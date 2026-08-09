-- +goose Up
-- Таблица аудита изменений полей промо-активностей.
-- Фиксирует: кто, что, когда изменил, старые и новые значения.

IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'tbl_AuditLog' AND TABLE_SCHEMA = 'dbo')
BEGIN
    CREATE TABLE dbo.tbl_AuditLog (
        id BIGINT IDENTITY PRIMARY KEY,
        entity_type NVARCHAR(50) NOT NULL DEFAULT 'promo',  -- promo, sales и т.д.
        entity_id INT NOT NULL,                              -- ID записи в tbl_PromoActivities
        user_name NVARCHAR(100) NOT NULL,                    -- Кто изменил (из JWT)
        action_type NVARCHAR(20) NOT NULL,                   -- CREATE, UPDATE, APPROVE, REJECT, DELETE, RESTORE
        changed_fields NVARCHAR(MAX),                        -- JSON: {"plan_units": {"old": 100, "new": 150}, ...}
        created_at DATETIME DEFAULT GETDATE()
    )
END;

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_AuditLog_entity')
    CREATE INDEX IX_AuditLog_entity ON dbo.tbl_AuditLog(entity_type, entity_id);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_AuditLog_created')
    CREATE INDEX IX_AuditLog_created ON dbo.tbl_AuditLog(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_AuditLog;