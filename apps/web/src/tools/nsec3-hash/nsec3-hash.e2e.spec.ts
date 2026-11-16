import { expect, test } from '@playwright/test'

test('nsec3-hash: matches the RFC 5155 example', async ({ page }) => {
  await page.goto('/nsec3-hash?domain=example&names=a&salt=aabbccdd&iterations=12')
  await expect(page.getByTestId('result')).toContainText('35mthgpgcu1qg68fab165klnsnk3dpvl.example')
})
