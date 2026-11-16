import { expect, test } from '@playwright/test'

test('dmarc-generator: builds a record and loads an existing one', async ({ page }) => {
  await page.goto('/dmarc-generator?domain=example.com&p=reject&rua=dmarc%40example.com')
  await expect(page.getByTestId('record-value').first()).toHaveText('v=DMARC1; p=reject; rua=mailto:dmarc@example.com')
  await page.getByTestId('import').locator('textarea').fill('v=DMARC1; p=quarantine; pct=50')
  await page.getByTestId('load').click()
  await expect(page.getByTestId('record-value').first()).toContainText('p=quarantine; pct=50')
})
