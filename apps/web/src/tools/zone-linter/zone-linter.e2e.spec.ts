import { expect, test } from '@playwright/test'

test('zone-linter: finds the mistakes in the example zone', async ({ page }) => {
  await page.goto('/zone-linter')
  await page.getByTestId('example').click()
  await expect(page.getByTestId('findings')).toContainText('CNAME')
  await expect(page.getByTestId('count-error')).not.toContainText('0 ')
})
