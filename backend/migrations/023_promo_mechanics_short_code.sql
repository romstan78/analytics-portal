-- +goose Up
-- Короткое обозначение механики промо для плиток витрины реестра.
--
-- Колонка живёт в справочнике механик, а не в отдельной таблице: канал уже
-- здесь, и вторая таблица с тем же каналом однажды разошлась бы с первой.
--
-- NULL разрешён намеренно. Механика, которой ещё не назначили код, не должна
-- ломать витрину: приложение в этом случае сокращает название само
-- (services.promoShortCode). Так новая механика в рабочей базе появляется
-- без миграции, а осмысленный код ей назначают уже потом.
IF COL_LENGTH('dbo.tbl_MechanicsChannelMapping', 'short_code') IS NULL
    ALTER TABLE dbo.tbl_MechanicsChannelMapping ADD short_code NVARCHAR(12) NULL;

-- Код обязан быть уникальным: две механики с одной плиткой означают метку,
-- которая врёт. Индекс фильтрованный — незаполненных кодов может быть много.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_MechanicsChannelMapping_short_code'
      AND object_id = OBJECT_ID('dbo.tbl_MechanicsChannelMapping')
)
    CREATE UNIQUE INDEX UX_MechanicsChannelMapping_short_code
        ON dbo.tbl_MechanicsChannelMapping(short_code)
        WHERE short_code IS NOT NULL;

-- +goose Down
IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_MechanicsChannelMapping_short_code'
      AND object_id = OBJECT_ID('dbo.tbl_MechanicsChannelMapping')
)
    DROP INDEX UX_MechanicsChannelMapping_short_code ON dbo.tbl_MechanicsChannelMapping;
IF COL_LENGTH('dbo.tbl_MechanicsChannelMapping', 'short_code') IS NOT NULL
    ALTER TABLE dbo.tbl_MechanicsChannelMapping DROP COLUMN short_code;
