import { expect, test } from '@playwright/test'

test('expiry-watchlist: adds a host, checks it and keeps it after reload', async ({ page }) => {
  await page.goto('/expiry-watchlist')
  await page.getByTestId('input').locator('textarea').fill('cert:example.com')
  await page.getByTestId('add').click()
  await expect(page.getByTestId('result')).toContainText('example.com', { timeout: 60_000 })
  await page.reload()
  await expect(page.getByTestId('entries')).toContainText('example.com')
})
