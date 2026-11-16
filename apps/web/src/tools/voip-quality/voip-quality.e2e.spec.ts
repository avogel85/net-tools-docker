import { expect, test } from '@playwright/test'

test('voip-quality: streams packets and scores the connection', async ({ page }) => {
  await page.goto('/voip-quality?dur=5')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('mos')).toHaveText(/^[1-4]\.\d\d$/, { timeout: 20_000 })
  await expect(page.getByTestId('error')).toHaveCount(0)
})
