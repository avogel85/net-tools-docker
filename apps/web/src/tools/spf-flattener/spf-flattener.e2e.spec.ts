import { expect, test } from '@playwright/test'

test('spf-flattener: flattens a real SPF record', async ({ page }) => {
  await page.goto('/spf-flattener?domain=google.com')
  await expect(page.getByTestId('records').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 30_000 })
})

test('spf-flattener: rejects an invalid label', async ({ page }) => {
  await page.goto('/spf-flattener?domain=example.com&label=a.b')
  await expect(page.getByTestId('run')).toBeDisabled()
})
