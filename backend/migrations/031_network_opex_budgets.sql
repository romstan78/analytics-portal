-- +goose Up
-- Бюджет OPEX по контракту: пять статей расходов, которые сеть оказывает как
-- услугу, а не зарабатывает объёмом.
--
-- До этой миграции реестр знал ровно один механизм инвестиций — процент от
-- товарооборота на квартальной строке бренда (investments_pct). Это бонус за
-- объём, то есть GTN; OPEX существовал только в карточках промо, в свободном
-- поле gtn_opex. Бюджет по договору вести было негде.
--
-- Ввод квартальный, хранение помесячное. Квартал — то, как бюджет согласуют;
-- месяц — то, в чём его потребляют: витрина, помесячный разрез и выгрузки
-- читают месяцы, и раскладывать квартал на чтении каждый раз заново значило бы
-- держать правило раскладки в нескольких местах.
--
-- Знак разрешён намеренно: бюджет правят задним числом, и возврат или
-- корректировка — это отрицательная сумма, а не отсутствие строки. Поэтому
-- CHECK (amount >= 0) здесь нет — в отличие от tbl_NetworkMonthlyFacts и
-- tbl_NetworkForecasts, где он стоит. Это решение, а не упущение: добавить его
-- «по аналогии с соседями» означает сломать корректировки.
--
-- Обе базы НДС хранятся всегда, тем же обещанием, что и у инвестиций GTN:
-- колонка «без НДС» пригодна для сложения сетей с разными ставками. У сети без
-- НДС в квартале они равны — это не дублирование, а гарантия потребителю.
-- Вводится сумма с НДС, «без НДС» считает сервер по ставке квартала.

IF OBJECT_ID('dbo.tbl_NetworkOpexBudgets', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkOpexBudgets (
        id              BIGINT IDENTITY(1,1) NOT NULL,
        network_id      INT NOT NULL,
        [year]          INT NOT NULL,
        [month]         INT NOT NULL,
        brand_as        NVARCHAR(255) NOT NULL,
        article         NVARCHAR(30) NOT NULL,
        amount_rub      DECIMAL(18,2) NOT NULL,
        amount_rub_net  DECIMAL(18,2) NOT NULL,
        updated_by      NVARCHAR(100) NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_NetworkOpexBudgets_created DEFAULT GETDATE(),
        updated_at      DATETIME NOT NULL CONSTRAINT DF_NetworkOpexBudgets_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkOpexBudgets PRIMARY KEY (id),
        CONSTRAINT FK_NetworkOpexBudgets_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT UQ_NetworkOpexBudgets_row UNIQUE (network_id, [year], [month], brand_as, article),
        CONSTRAINT CK_NetworkOpexBudgets_month CHECK ([month] BETWEEN 1 AND 12),
        -- Список статей закрыт контрактом. Новая статья — новая миграция: сверять
        -- свободный текст в пяти местах дороже, чем добавить значение осознанно.
        CONSTRAINT CK_NetworkOpexBudgets_article CHECK (article IN (
            'ntz_bdn', 'display', 'reports', 'fixed_promo', 'product_card'
        ))
    )
END;

-- Покрывающий индекс года: и карточка, и витрина читают бюджет сети за год
-- целиком, а не по одному месяцу.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkOpexBudgets_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkOpexBudgets')
)
    CREATE INDEX IX_NetworkOpexBudgets_network_year
        ON dbo.tbl_NetworkOpexBudgets(network_id, [year])
        INCLUDE ([month], brand_as, article, amount_rub, amount_rub_net, updated_at);

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_NetworkOpexBudgets;
