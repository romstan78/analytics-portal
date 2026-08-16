import { test, expect } from '@playwright/test';

test('страница входа отображается', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('text=Вход в систему')).toBeVisible();
  await expect(page.locator('label:has-text("Логин")')).toBeVisible();
  await expect(page.locator('label:has-text("Пароль")')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Войти' })).toBeVisible();
});

test('ошибка при пустых полях', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.locator('text=Заполните все поля')).toBeVisible();
});

test('ошибка при неверных данных', async ({ page }) => {
  await page.goto('/');
  await page.locator('input[type="text"]').fill('wrong');
  await page.locator('input[type="password"]').fill('wrong');
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('alert')).toContainText(
    /неверный логин|Ошибка входа|Сервер недоступен/,
    { timeout: 10000 },
  );
});
