import { expect, test } from '@playwright/test'

test('prefix-watch: adds a prefix and checks it', async ({ page }) => {
  await page.goto('/prefix-watch')
  await page.getByTestId('input').locator('textarea').fill('1.1.1.0/24 AS13335')
  await page.getByTestId('add').click()
  await expect(page.getByTestId('result')).toContainText('1.1.1.0/24')
  await expect(page.getByTestId('result')).toContainText('AS13335', { timeout: 60_000 })
})
