import { expect, test } from '@playwright/test'

test('pq-tls-check: checks a well-known server', async ({ page }) => {
  await page.goto('/pq-tls-check?host=www.cloudflare.com')
  await expect(page.getByTestId('verdict').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
