import { expect, test } from '@playwright/test'

test('reverse-proxy-config: renders an nginx server block from the URL', async ({ page }) => {
  await page.goto('/reverse-proxy-config?hosts=shop.example.com&upstream=http%3A%2F%2F10.0.0.5%3A8080')
  await expect(page.getByTestId('config-nginx')).toContainText('server_name shop.example.com;')
  await expect(page.getByTestId('config-nginx')).toContainText('proxy_pass http://10.0.0.5:8080;')
})
