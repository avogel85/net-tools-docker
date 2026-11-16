import { expect, test } from '@playwright/test'

test('reverse-traceroute: detects the public IP and offers the trace', async ({ page }) => {
  await page.route('https://api.ipify.org/**', route => route.fulfill({ json: { ip: '203.0.113.50' } }))
  await page.route('https://api6.ipify.org/**', route => route.abort())
  await page.goto('/reverse-traceroute')
  await expect(page.getByText('203.0.113.50')).toBeVisible()
  await expect(page.getByTestId('run')).toBeEnabled()
})
