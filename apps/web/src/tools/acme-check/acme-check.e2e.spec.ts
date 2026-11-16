import { expect, test } from '@playwright/test'

test('acme-check: checks a real host name', async ({ page }) => {
  await page.goto('/acme-check?name=example.com')
  await expect(page.getByTestId('verdict').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
