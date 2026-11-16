import { expect, test } from '@playwright/test'

test('ike-probe: probes a host', async ({ page }) => {
  await page.goto('/ike-probe?host=example.com')
  await expect(page.getByTestId('verdict').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
