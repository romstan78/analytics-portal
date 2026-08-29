-- +goose Up
-- Три показателя инвестиций, каждый в двух базах НДС, становятся колонками.
--
-- До этого в квартальной строке жили только две суммы, и обе — не то, чем
-- казались: плановые инвестиции не хранились вовсе (считались на лету), базы
-- «без НДС» не было ни у одной, а fact_investments_rub держал перечисленное
-- по документам, выдавая его за фактические инвестиции по договору.
--
-- Теперь:
--   plan_investments_rub      / _net — процент от планового объёма, без порога;
--   forecast_investments_rub  / _net — процент от прогноза, если прогноз закрыл план;
--   fact_investments_rub      / _net — процент от факта, если факт закрыл план;
--   paid_investments_rub            — сколько реально перечислено по документам.
--
-- Последняя колонка не показатель правила, а платёжный факт. Она нужна там, где
-- считают доплату: вычесть уже перечисленное из начисленного правилом больше
-- нечем, и путать её с fact_investments_rub нельзя.
--
-- Обе базы НДС заполняются всегда. У сети без НДС они равны — это обещание
-- потребителю колонок, а не совпадение: «без НДС» всегда пригодна для сложения
-- сетей с разными ставками.

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'plan_investments_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD plan_investments_rub DECIMAL(18,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'plan_investments_rub_net') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD plan_investments_rub_net DECIMAL(18,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_investments_rub_net') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD forecast_investments_rub_net DECIMAL(18,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_investments_rub_net') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD fact_investments_rub_net DECIMAL(18,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'paid_investments_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD paid_investments_rub DECIMAL(18,2) NULL;

-- +goose StatementBegin
-- Перечисленное по документам переезжает под своё имя. До этой миграции его
-- писал в fact_investments_rub загрузчик отгрузок — единственный источник этой
-- колонки, поэтому перенос однозначен.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'paid_investments_rub') IS NOT NULL
BEGIN
    UPDATE dbo.tbl_NetworkPlans
       SET paid_investments_rub = fact_investments_rub
     WHERE fact_investments_rub IS NOT NULL
       AND paid_investments_rub IS NULL;
END
-- +goose StatementEnd

-- Расчётные колонки заполняются пересчётом (cmd/recalc_investments), а не этой
-- миграцией: правило смотрит шире одной строки — на валовый пул и на правила
-- совместного зачёта, — и выразить его одним UPDATE нельзя, не соврав.
UPDATE dbo.tbl_NetworkPlans
   SET fact_investments_rub = NULL,
       forecast_investments_rub = NULL
 WHERE investments_pct IS NOT NULL;

-- Покрывающий индекс сетки: инвестиции читаются вместе с остальными суммами.
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;
CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub,
             investments_pct, plan_investments_rub, plan_investments_rub_net,
             forecast_investments_rub, forecast_investments_rub_net,
             fact_investments_rub, fact_investments_rub_net, paid_investments_rub);

-- +goose Down
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

-- Возвращаем перечисленное туда, где его ищет старый код.
UPDATE dbo.tbl_NetworkPlans
   SET fact_investments_rub = paid_investments_rub
 WHERE paid_investments_rub IS NOT NULL;

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'plan_investments_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN plan_investments_rub;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'plan_investments_rub_net') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN plan_investments_rub_net;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_investments_rub_net') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN forecast_investments_rub_net;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_investments_rub_net') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN fact_investments_rub_net;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'paid_investments_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN paid_investments_rub;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub);
