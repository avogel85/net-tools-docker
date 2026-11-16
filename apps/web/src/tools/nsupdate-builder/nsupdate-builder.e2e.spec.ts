import { expect, test } from '@playwright/test'

test('nsupdate-builder: generates a TSIG key and builds a script', async ({ page }) => {
  await page.goto('/nsupdate-builder?zone=example.com&server=ns1.example.com&ops=add%20www%20300%20A%20192.0.2.1')
  await page.getByTestId('generate').click()
  await expect(page.getByTestId('config-bind')).toContainText('algorithm hmac-sha256;')
  await expect(page.getByTestId('script')).toContainText('update add www.example.com. 300 A 192.0.2.1')
  await expect(page).not.toHaveURL(/secret/)
})
