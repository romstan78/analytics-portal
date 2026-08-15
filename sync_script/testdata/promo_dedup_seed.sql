SET XACT_ABORT ON;
BEGIN TRANSACTION;

DECLARE @exact TABLE (id INT);
INSERT INTO dbo.tbl_PromoActivities
    (network_name, [year], [month], sku, mechanics, conditions, plan_promo_units, created_by, updated_by)
OUTPUT inserted.id INTO @exact(id)
VALUES
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-EXACT', N'Discount', N'Same', 100, N'test', N'test'),
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-EXACT', N'Discount', N'Same', 100, N'test', N'test');

DECLARE @exact_min INT = (SELECT MIN(id) FROM @exact);
DECLARE @exact_max INT = (SELECT MAX(id) FROM @exact);

INSERT INTO dbo.tbl_PromoComments (promo_id, user_name, role, comment_text)
VALUES (@exact_min, N'tester', N'admin', N'Comment to move');

INSERT INTO dbo.tbl_AuditLog (entity_type, entity_id, user_name, action_type, changed_fields)
VALUES
    ('promo', @exact_min, N'tester', 'UPDATE', N'{}'),
    ('promo', @exact_max, N'tester', 'UPDATE', N'{}'),
    ('promo', @exact_max, N'tester', 'UPDATE', N'{}');

INSERT INTO dbo.tbl_PromoActivities
    (network_name, [year], [month], sku, mechanics, conditions, plan_promo_units, created_by, updated_by)
VALUES
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-EXACT-2', N'Discount', N'Same', 200, N'test', N'test'),
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-EXACT-2', N'Discount', N'Same', 200, N'test', N'test');

INSERT INTO dbo.tbl_PromoActivities
    (network_name, [year], [month], sku, mechanics, conditions, plan_promo_units,
     agreement1, agreement1_status, created_by, updated_by)
VALUES
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-APPROVAL', N'Discount', N'Same', 300, NULL, NULL, N'test', N'test'),
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-APPROVAL', N'Discount', N'Same', 300, N'согласовано', 'approved', N'test', N'test');

INSERT INTO dbo.tbl_PromoActivities
    (network_name, [year], [month], sku, mechanics, conditions, plan_promo_units, created_by, updated_by)
VALUES
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-DATA', N'Discount', N'Condition A', 400, N'test', N'test'),
    (N'TEST-DEDUPE-NETWORK', 2026, 8, N'TEST-DEDUPE-DATA', N'Discount', N'Condition B', 400, N'test', N'test');

COMMIT TRANSACTION;
