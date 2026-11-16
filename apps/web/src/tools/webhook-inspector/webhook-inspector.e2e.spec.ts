import { expect, test } from '@playwright/test'

test('webhook-inspector: captures a request sent to the bin', async ({ page, request }) => {
  await page.goto('/webhook-inspector')
  await page.getByTestId('create').click()
  const url = (await page.getByTestId('hook-url').textContent())!.trim()
  await request.post(`${url}/events?x=1`, { data: { hello: 'world' } })
  await expect(page.getByTestId('request').first()).toContainText('POST /events?x=1', { timeout: 15_000 })
  await expect(page.getByTestId('result')).toContainText('"hello": "world"')
  await page.getByTestId('delete').click()
  await expect(page.getByTestId('create')).toBeVisible()
})
