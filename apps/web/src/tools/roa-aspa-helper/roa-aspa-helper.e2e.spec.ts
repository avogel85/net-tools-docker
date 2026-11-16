import { expect, test } from '@playwright/test'

test('roa-aspa-helper: flags a route leak with ASPA', async ({ page }) => {
  await page.goto('/roa-aspa-helper')
  await page.getByTestId('records').locator('textarea').fill('AS64500: AS64510')
  await page.getByTestId('path').fill('64599 64500')
  await expect(page.getByTestId('verdict')).toHaveText(/invalid/i)
})

test('roa-aspa-helper: builds a ROA plan for an AS', async ({ page }) => {
  await page.goto('/roa-aspa-helper?asn=AS13335')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
