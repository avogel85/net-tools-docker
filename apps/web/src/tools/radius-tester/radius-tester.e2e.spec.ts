import { expect, test } from '@playwright/test'

test('radius-tester: keeps the secret out of the URL', async ({ page }) => {
  await page.goto('/radius-tester?host=192.0.2.10')
  const disabled = page.getByTestId('tool-disabled')
  if (await disabled.isVisible().catch(() => false))
    return // LAN profile off: the tool explains how to enable it
  await page.getByTestId('secret').locator('input').fill('topsecret')
  await expect(page).not.toHaveURL(/topsecret/)
})
