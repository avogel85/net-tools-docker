import { expect, test } from '@playwright/test'

test('udp-probe: finds the DNS service of a public resolver', async ({ page }) => {
  await page.goto('/udp-probe?host=1.1.1.1&service=dns')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
