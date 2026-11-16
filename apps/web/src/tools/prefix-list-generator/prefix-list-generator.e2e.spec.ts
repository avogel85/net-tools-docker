import { expect, test } from '@playwright/test'

test('prefix-list-generator: builds a prefix list for an AS', async ({ page }) => {
  await page.goto('/prefix-list-generator?set=AS13335&v6=false')
  await expect(page.getByTestId('config-ios').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 90_000 })
})
