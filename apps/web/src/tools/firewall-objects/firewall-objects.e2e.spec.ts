import { expect, test } from '@playwright/test'

test('firewall-objects: converts a list for FortiGate', async ({ page }) => {
  await page.goto('/firewall-objects?list=192.0.2.0%2F24%0A198.51.100.7&name=BLOCK')
  await expect(page.getByTestId('output-fortigate')).toContainText('set subnet 192.0.2.0 255.255.255.0')
})
