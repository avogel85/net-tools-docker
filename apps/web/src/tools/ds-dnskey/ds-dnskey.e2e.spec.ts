import { expect, test } from '@playwright/test'

test('ds-dnskey: computes a DS record from a pasted DNSKEY', async ({ page }) => {
  await page.goto('/ds-dnskey?mode=paste')
  await page.getByTestId('owner').fill('example.com')
  await page.getByTestId('dnskeys').locator('textarea').fill('257 3 13 kXKkvWU3vGYfTJGl3qBd4qhiWp5aRs7YtkCJxD2d+t7KXqwahww5IgJtxJT2yFItlggazyfXqJEVOmMJ3qT0tQ==')
  await page.getByTestId('compute').click()
  await expect(page.getByTestId('keys')).toContainText('13 2 ')
})

test('ds-dnskey: checks a signed domain', async ({ page }) => {
  await page.goto('/ds-dnskey?domain=example.com')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('verdict').or(page.getByTestId('api-error')).or(page.getByTestId('resolver-error'))).toBeVisible({ timeout: 30_000 })
})
