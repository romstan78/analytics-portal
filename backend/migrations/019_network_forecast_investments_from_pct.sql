-- +goose Up
-- Прогноз инвестиций перестаёт быть вводимой величиной: он считается процентом
-- бренда от EAC объёма. В forecast_investments_rub остаётся только осознанное
-- переопределение КАМа — разовая выплата вне процента.
--
-- Поэтому снимаем два вида значений, которые переопределениями не являются:
--   * строки миграции 012 — это механический пересчёт forecast_rub * investments_pct,
--     то есть ровно то, что теперь считается на лету. Оставь их — и любая сеть
--     навсегда застынет на инвестициях, посчитанных от прогноза 2026 года;
--   * все SKU-строки — процент инвестиций ведётся по бренду, по SKU его нет,
--     и суммирование SKU в бренд давало вторую, несогласованную сумму.
--
-- Значения, введённые людьми на уровне бренда, сохраняются: форма покажет их
-- как переопределение с возможностью вернуться к проценту.

UPDATE dbo.tbl_NetworkForecasts
   SET forecast_investments_rub = NULL,
       updated_at = GETDATE()
 WHERE forecast_investments_rub IS NOT NULL
   AND (sku IS NOT NULL OR updated_by = 'migration-012');

-- Квартальная витрина хранит скатанный EAC инвестиций. Там, где он был получен
-- из тех же снятых значений, пересчёт вернёт его при первом открытии прогноза;
-- до тех пор строка не должна выдавать старую сумму за актуальную.
UPDATE p
   SET p.forecast_investments_rub = NULL,
       p.updated_at = GETDATE()
FROM dbo.tbl_NetworkPlans p
WHERE p.forecast_investments_rub IS NOT NULL
  AND p.investments_pct IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
        FROM dbo.tbl_NetworkForecasts f
       WHERE f.network_id = p.network_id
         AND f.[year] = p.[year]
         AND f.[month] BETWEEN (p.[quarter] - 1) * 3 + 1 AND (p.[quarter] - 1) * 3 + 3
         AND f.brand_as = p.brand_as
         AND f.sku IS NULL
         AND f.forecast_investments_rub IS NOT NULL
  );

-- +goose Down
-- Снятые значения не восстанавливаются: исходные суммы были расчётными и
-- воспроизводятся процентом от прогноза.
SELECT 1;
