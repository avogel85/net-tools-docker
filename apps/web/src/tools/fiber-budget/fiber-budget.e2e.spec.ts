import { expect, test } from '@playwright/test'

test('fiber-budget: computes a 10GBASE-LR link', async ({ page }) => {
  await page.goto('/fiber-budget?length=8000&splices=4&conndb=0.5')
  await expect(page.getByTestId('r-totalLoss')).toContainText('4.60 dB')
  await expect(page.getByTestId('verdict')).toBeVisible()
})
