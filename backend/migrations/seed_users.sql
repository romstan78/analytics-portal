-- Seed: пользователи с реальными bcrypt-хешами (cost=10)
-- Пароли: manager1/promo2024!, manager2/promo2024!, admin/admin2024!

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager1' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager1', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement1');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'manager2' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('manager2', '$2a$10$De13I4p4Zrp5LkmtzfgnXOH1RyMUI9rASk.VHJNDcrd/neBQQotk2', 'agreement2');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE username = 'admin' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_Users (username, password_hash, role) VALUES ('admin', '$2a$10$jyr5S3OrcUK5UmgUwsnDCeUjhEHdcQji7tO.L.y0oAncVBA/YTzFO', 'admin');