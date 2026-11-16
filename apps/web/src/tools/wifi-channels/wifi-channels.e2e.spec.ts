import { expect, test } from '@playwright/test'

test('wifi-channels: plans 2.4 GHz access points on 1/6/11', async ({ page }) => {
  await page.goto('/wifi-channels?band=2.4&width=20&aps=3')
  await expect(page.getByTestId('assignment')).toHaveText(['AP 1: 1', 'AP 2: 6', 'AP 3: 11'])
})
