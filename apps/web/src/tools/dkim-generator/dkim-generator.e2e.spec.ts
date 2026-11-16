import { expect, test } from '@playwright/test'

test('dkim-generator: generates a key pair and selector record', async ({ page }) => {
  await page.goto('/dkim-generator?domain=example.com&selector=s1')
  await page.getByTestId('run').click()
  await expect(page.getByTestId('record-value').first()).toContainText('v=DKIM1; k=rsa; p=MII')
  await expect(page.getByTestId('private')).toContainText('BEGIN PRIVATE KEY')
})
