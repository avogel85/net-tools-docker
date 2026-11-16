import { expect, test } from '@playwright/test'

test('capture-filter: builds tcpdump and Wireshark filters', async ({ page }) => {
  await page.goto('/capture-filter?hosts=192.0.2.10&ports=443&proto=tcp')
  await expect(page.getByTestId('out-bpf')).toHaveText('tcp and host 192.0.2.10 and tcp port 443')
  await expect(page.getByTestId('out-display')).toHaveText('tcp && ip.addr == 192.0.2.10 && tcp.port == 443')
})
