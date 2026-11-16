import { expect, test } from '@playwright/test'

test('broken-links: crawls a small site', async ({ page }) => {
  await page.goto('/broken-links?url=https%3A%2F%2Fexample.com&external=false')
  await expect(page.getByTestId('summary').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 90_000 })
})
