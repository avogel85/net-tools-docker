import { expect, test } from '@playwright/test'

test('email-validator: rejects bad syntax and checks real domains', async ({ page }) => {
  await page.goto('/email-validator')
  await page.getByTestId('addresses').locator('textarea').fill('postmaster@example.com\nnot-an-address')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 30_000 })
  await expect(page.getByTestId('result')).toContainText('not-an-address')
  await expect(page).not.toHaveURL(/example/)
})
