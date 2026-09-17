-- +goose Up
-- Статус промо сводится к четырём значениям и дальше выводится сервером
-- (repository.promoStatusCaseSQL): «В процессе согласования» → «Финализировано»
-- (оба согласования) → «Проведено» (внесён факт); «Отклонено» — оба отклонены.
-- Историю не откатываем: «проведено» и «финализировано» остаются таковыми, как бы
-- ни были заполнены новые колонки согласования — их у старых строк нет. Прочие
-- значения (NULL, пусто, «Планируется», «В процессе») выводятся по правилу.
-- CHECK-ограничения нет намеренно: импорт из Excel и демо-копия пишут статус
-- напрямую и не обязаны знать про нормализацию.
UPDATE dbo.tbl_PromoActivities SET status = CASE
    WHEN agreement1_status = 'rejected' AND agreement2_status = 'rejected' THEN N'Отклонено'
    WHEN LOWER(LTRIM(RTRIM(ISNULL(status, '')))) = N'проведено' THEN N'Проведено'
    WHEN LOWER(LTRIM(RTRIM(ISNULL(status, '')))) = N'финализировано' THEN N'Финализировано'
    WHEN LOWER(LTRIM(RTRIM(ISNULL(status, '')))) = N'в процессе согласования' THEN N'В процессе согласования'
    WHEN actual_promo_sales_units IS NOT NULL AND actual_investments IS NOT NULL THEN N'Проведено'
    WHEN agreement1_status = 'approved' AND agreement2_status = 'approved' THEN N'Финализировано'
    ELSE N'В процессе согласования' END
-- Сравнение регистрозависимое (COLLATE): коллация базы регистр не различает,
-- и «проведено» иначе сошло бы за уже нормальное значение, а Go сравнивает
-- строки побайтно.
WHERE status IS NULL
   OR status COLLATE Cyrillic_General_CS_AS NOT IN (N'В процессе согласования', N'Финализировано', N'Проведено', N'Отклонено')
   OR (agreement1_status = 'rejected' AND agreement2_status = 'rejected');

-- +goose Down
-- Исходные значения статусов не сохранялись; откат данных невозможен.
SELECT 1;
