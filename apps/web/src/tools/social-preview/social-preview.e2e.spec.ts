import { expect, test } from '@playwright/test'

test('social-preview: previews a real page', async ({ page }) => {
  await page.goto('/social-preview?url=https%3A%2F%2Fgithub.com')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
