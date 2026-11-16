import { expect, test } from '@playwright/test'

test('dc-locator: reports a domain without AD records', async ({ page }) => {
  await page.goto('/dc-locator?domain=example.com&resolver=cloudflare')
  await expect(page.getByTestId('findings').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 30_000 })
  await expect(page.getByTestId('findings')).toContainText('_ldap._tcp.dc._msdcs')
})
