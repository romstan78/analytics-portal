-- Таблица комментариев к промо-активностям
-- Заменяет текстовое поле comments в tbl_PromoActivities
CREATE TABLE dbo.tbl_PromoComments (
    id BIGINT IDENTITY PRIMARY KEY,
    promo_id INT NOT NULL,
    user_name NVARCHAR(100) NOT NULL,
    role NVARCHAR(50) NOT NULL,       -- 'КАМ', 'согласование1', 'согласование2', 'admin'
    comment_text NVARCHAR(MAX) NOT NULL,
    created_at DATETIME DEFAULT GETDATE(),
    CONSTRAINT FK_PromoComments_Promo FOREIGN KEY (promo_id) REFERENCES dbo.tbl_PromoActivities(id)
);

CREATE INDEX IX_PromoComments_promo_id ON dbo.tbl_PromoComments(promo_id);