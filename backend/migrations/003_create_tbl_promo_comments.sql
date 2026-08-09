-- +goose Up
-- Таблица комментариев к промо-активностям.
-- Заменяет текстовое поле comments в tbl_PromoActivities.

IF NOT EXISTS (SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'tbl_PromoComments' AND TABLE_SCHEMA = 'dbo')
BEGIN
    CREATE TABLE dbo.tbl_PromoComments (
        id BIGINT IDENTITY PRIMARY KEY,
        promo_id INT NOT NULL,
        user_name NVARCHAR(100) NOT NULL,
        role NVARCHAR(50) NOT NULL,       -- 'КАМ', 'согласование1', 'согласование2', 'admin'
        comment_text NVARCHAR(MAX) NOT NULL,
        created_at DATETIME DEFAULT GETDATE(),
        CONSTRAINT FK_PromoComments_Promo FOREIGN KEY (promo_id) REFERENCES dbo.tbl_PromoActivities(id)
    )
END;

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_PromoComments_promo_id')
    CREATE INDEX IX_PromoComments_promo_id ON dbo.tbl_PromoComments(promo_id);

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_PromoComments;