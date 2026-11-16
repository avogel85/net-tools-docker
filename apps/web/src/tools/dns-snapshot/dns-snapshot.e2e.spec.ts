import { expect, test } from '@playwright/test'

test('dns-snapshot: takes two snapshots and compares them', async ({ page }) => {
  await page.goto('/dns-snapshot?domain=example.com')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('result')).toBeVisible({ timeout: 60_000 })
  await page.getByTestId('run').click()
  await expect(page.getByTestId('no-changes').or(page.getByTestId('diff'))).toBeVisible({ timeout: 60_000 })
  await expect(page.getByTestId('saved')).toContainText('(2)')
})
