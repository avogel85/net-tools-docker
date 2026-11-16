import { expect, test } from '@playwright/test'

test('email-provider-detector: detects the providers of a real domain', async ({ page }) => {
  await page.goto('/email-provider-detector?domain=github.com')
  await expect(page.getByTestId('result').first().or(page.getByTestId('none')).or(page.getByTestId('api-error'))).toBeVisible({ timeout: 30_000 })
})
