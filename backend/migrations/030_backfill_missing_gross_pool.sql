-- +goose Up
-- До исправления первый перевод бренда в валовый объём не создавал строку
-- пула (brand_as IS NULL). Флаги in_gross сохранялись, но одиночные итоги не
-- могли распознать валовый контракт. Восстанавливаем такой пул суммой валовых
-- брендов: общее обязательство при этом не меняется, остаток равен нулю.
--
-- Метка updated_by делает Down точечным: пользовательские строки пула он не
-- затрагивает, а изменённая после миграции строка уже считается пользовательской.
INSERT INTO dbo.tbl_NetworkPlans (
    network_id, [year], [quarter], brand_as, in_gross, plan_rub,
    investments_pct, updated_by
)
SELECT
    p.network_id,
    p.[year],
    p.[quarter],
    NULL,
    0,
    SUM(p.plan_rub),
    NULL,
    N'migration-030-gross-pool-backfill'
FROM dbo.tbl_NetworkPlans p
WHERE p.brand_as IS NOT NULL
  AND p.in_gross = 1
GROUP BY p.network_id, p.[year], p.[quarter]
HAVING COUNT(p.plan_rub) > 0
   AND NOT EXISTS (
       SELECT 1
       FROM dbo.tbl_NetworkPlans pool
       WHERE pool.network_id = p.network_id
         AND pool.[year] = p.[year]
         AND pool.[quarter] = p.[quarter]
         AND pool.brand_as IS NULL
   );

-- +goose Down
DELETE FROM dbo.tbl_NetworkPlans
WHERE brand_as IS NULL
  AND updated_by = N'migration-030-gross-pool-backfill';
