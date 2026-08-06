-- Таблица аудита изменений полей промо-активностей
-- Фиксирует: кто, что, когда изменил, старые и новые значения
CREATE TABLE dbo.tbl_AuditLog (
    id BIGINT IDENTITY PRIMARY KEY,
    entity_type NVARCHAR(50) NOT NULL DEFAULT 'promo',  -- promo, sales и т.д.
    entity_id INT NOT NULL,                              -- ID записи в tbl_PromoActivities
    user_name NVARCHAR(100) NOT NULL,                    -- Кто изменил (из JWT)
    action_type NVARCHAR(20) NOT NULL,                   -- CREATE, UPDATE, APPROVE, REJECT, DELETE
    changed_fields NVARCHAR(MAX),                        -- JSON: {"plan_units": {"old": 100, "new": 150}, ...}
    created_at DATETIME DEFAULT GETDATE()
);

CREATE INDEX IX_AuditLog_entity ON dbo.tbl_AuditLog(entity_type, entity_id);
CREATE INDEX IX_AuditLog_created ON dbo.tbl_AuditLog(created_at DESC);