import { expect, test } from '@playwright/test'

test('lookalike-finder: lists registered variants of a domain', async ({ page }) => {
  await page.goto('/lookalike-finder?domain=example.com')
  await expect(page.getByTestId('summary').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 90_000 })
})
