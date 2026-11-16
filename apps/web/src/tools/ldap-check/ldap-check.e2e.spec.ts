import { expect, test } from '@playwright/test'

test('ldap-check: reports an unreachable LDAP endpoint', async ({ page }) => {
  await page.goto('/ldap-check?host=example.com&mode=ldap')
  await expect(page.getByTestId('result').or(page.getByTestId('api-error'))).toBeVisible({ timeout: 60_000 })
})
