import { expect, test } from '@playwright/test'

test('dkim-verifier: reports an unsigned message', async ({ page }) => {
  await page.goto('/dkim-verifier')
  await page.getByTestId('message').locator('textarea').fill('From: a@example.com\nTo: b@example.net\nSubject: test\n\nhello\n')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('verdict')).toContainText('DKIM-Signature')
  await expect(page).not.toHaveURL(/hello/)
})
