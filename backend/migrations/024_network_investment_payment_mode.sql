-- +goose Up
-- +goose StatementBegin
-- Для каждой строки «бренд × квартал» можно выбрать безусловную оплату от
-- фактического объёма. По умолчанию действует обычное правило: EAC и порог
-- выполнения плана 100% за квартал либо объединённый период.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'pay_investments_from_fact') IS NULL
BEGIN
    ALTER TABLE dbo.tbl_NetworkPlans
        ADD pay_investments_from_fact BIT NOT NULL
            CONSTRAINT DF_NetworkPlans_pay_investments_from_fact DEFAULT 0;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'pay_investments_from_fact') IS NOT NULL
BEGIN
    ALTER TABLE dbo.tbl_NetworkPlans
        DROP CONSTRAINT IF EXISTS DF_NetworkPlans_pay_investments_from_fact;
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN pay_investments_from_fact;
END;
-- +goose StatementEnd
