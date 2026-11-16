import { expect, test } from '@playwright/test'

test('wifi-throughput: shows the Wi-Fi 6 80 MHz 2x2 MCS 11 rate', async ({ page }) => {
  await page.goto('/wifi-throughput?std=ax&width=80&ss=2&mcs=11')
  await expect(page.getByTestId('phy')).toHaveText('1201 Mbit/s')
})
