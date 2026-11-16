import { expect, test } from '@playwright/test'

test('cabling-reference: filters the tables', async ({ page }) => {
  await page.goto('/cabling-reference?q=om4')
  await expect(page.getByTestId('section-fiber')).toContainText('4700 MHz·km')
  await expect(page.getByTestId('section-copper')).toHaveCount(0)
})
