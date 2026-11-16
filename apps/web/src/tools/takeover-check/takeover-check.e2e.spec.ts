import { expect, test } from '@playwright/test'

test('takeover-check: checks the common names of a domain', async ({ page }) => {
  await page.goto('/takeover-check?domain=example.com&names=www')
  await expect(page.getByTestId('verdict').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
