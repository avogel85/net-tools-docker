import { expect, test } from '@playwright/test'

test('sip-check: probes a host', async ({ page }) => {
  await page.goto('/sip-check?host=example.com&transport=tcp')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
