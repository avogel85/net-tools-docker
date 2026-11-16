import { expect, test } from '@playwright/test'

test('log-parser: recognises the example lines', async ({ page }) => {
  await page.goto('/log-parser')
  await page.getByTestId('example').click()
  await expect(page.getByTestId('result')).toContainText('Cisco ASA')
  await expect(page.getByTestId('line').first()).toContainText('RFC 5424')
})
