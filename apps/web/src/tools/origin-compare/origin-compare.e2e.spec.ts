import { expect, test } from '@playwright/test'

test('origin-compare: rejects a host name as origin', async ({ page }) => {
  await page.goto('/origin-compare?url=https%3A%2F%2Fexample.com&origin=origin.example.com')
  await expect(page.getByTestId('run')).toBeDisabled()
})
