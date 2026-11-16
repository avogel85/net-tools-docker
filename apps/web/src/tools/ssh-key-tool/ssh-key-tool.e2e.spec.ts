import { expect, test } from '@playwright/test'

test('ssh-key-tool: fingerprints a key and builds SSHFP records', async ({ page }) => {
  await page.goto('/ssh-key-tool?host=server.example.com')
  await page.getByTestId('keys').locator('textarea').fill('ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@host')
  await expect(page.getByTestId('fingerprint').first()).toContainText('SHA256:')
  await expect(page.getByTestId('sshfp')).toContainText('server.example.com')
  await expect(page.getByTestId('sshfp')).toContainText('4 2 ')
})
