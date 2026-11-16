import { expect, test } from '@playwright/test'

test('remote-access-check: checks a host', async ({ page }) => {
  await page.goto('/remote-access-check?host=example.com&service=ssh')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
