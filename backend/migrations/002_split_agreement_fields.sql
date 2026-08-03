-- Migration: разделение agreement1/agreement2 на status + comment
-- Заменить CHARINDEX-парсинг на нормальные колонки.

-- Добавляем новые колонки (если ещё не добавлены)
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement1_status') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement1_status NVARCHAR(20) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement1_comment') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement1_comment NVARCHAR(MAX) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement2_status') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement2_status NVARCHAR(20) NULL;
IF COL_LENGTH('dbo.tbl_PromoActivities', 'agreement2_comment') IS NULL
    ALTER TABLE dbo.tbl_PromoActivities ADD agreement2_comment NVARCHAR(MAX) NULL;
GO

-- Миграция существующих данных: парсим старые текстовые поля
-- approved: начинается с "согласовано"
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'approved',
    agreement1_comment = CASE WHEN agreement1 LIKE N'согласовано: %'
      THEN SUBSTRING(agreement1, CHARINDEX(N':', agreement1) + 2, LEN(agreement1))
      ELSE NULL END
WHERE agreement1 IS NOT NULL
  AND CHARINDEX(N'согласовано', agreement1) = 1
  AND agreement1_status IS NULL;

-- rejected: начинается с "отклонено"
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'rejected',
    agreement1_comment = CASE WHEN agreement1 LIKE N'отклонено: %'
      THEN SUBSTRING(agreement1, CHARINDEX(N':', agreement1) + 2, LEN(agreement1))
      ELSE NULL END
WHERE agreement1 IS NOT NULL
  AND CHARINDEX(N'отклонено', agreement1) = 1
  AND agreement1_status IS NULL;

-- commented: всё остальное не-NULL
UPDATE dbo.tbl_PromoActivities
SET agreement1_status = 'commented',
    agreement1_comment = agreement1
WHERE agreement1 IS NOT NULL
  AND agreement1_status IS NULL;

-- То же для agreement2
UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'approved',
    agreement2_comment = CASE WHEN agreement2 LIKE N'согласовано: %'
      THEN SUBSTRING(agreement2, CHARINDEX(N':', agreement2) + 2, LEN(agreement2))
      ELSE NULL END
WHERE agreement2 IS NOT NULL
  AND CHARINDEX(N'согласовано', agreement2) = 1
  AND agreement2_status IS NULL;

UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'rejected',
    agreement2_comment = CASE WHEN agreement2 LIKE N'отклонено: %'
      THEN SUBSTRING(agreement2, CHARINDEX(N':', agreement2) + 2, LEN(agreement2))
      ELSE NULL END
WHERE agreement2 IS NOT NULL
  AND CHARINDEX(N'отклонено', agreement2) = 1
  AND agreement2_status IS NULL;

UPDATE dbo.tbl_PromoActivities
SET agreement2_status = 'commented',
    agreement2_comment = agreement2
WHERE agreement2 IS NOT NULL
  AND agreement2_status IS NULL;

-- После миграции старые колонки можно оставить для обратной совместимости,
-- но бэкенд должен писать в новые поля.