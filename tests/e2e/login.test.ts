import {test, expect} from '@playwright/test';
import {login, logout} from './utils.ts';

test('homepage', async ({page}) => {
  await page.goto('/');
  await expect(page.locator('#navbar-logo img')).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('login and logout', async ({page}) => {
  await login(page);
  await logout(page);
});
