import { expect, test } from '@playwright/test'

test('mta-sts-generator: builds policy and records', async ({ page }) => {
  await page.goto('/mta-sts-generator?domain=example.com&mode=enforce&mx=mx1.example.com&id=20261116')
  await expect(page.getByTestId('record-value').first()).toHaveText('v=STSv1; id=20261116')
  await expect(page.getByTestId('policy')).toContainText('mta-sts.example.com/.well-known/mta-sts.txt')
})
