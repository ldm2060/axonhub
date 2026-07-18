import { expect, test } from '@playwright/test'
import { gotoAndEnsureAuth } from './auth.utils'

test.describe('Kimi Code OAuth channel form', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/admin/channels')
    await page.getByTestId('channels-table').waitFor({ state: 'visible', timeout: 15000 })
  })

  test('hides endpoint and standard API key fields', async ({ page }) => {
    await page.getByTestId('add-channel-button').click()

    const dialog = page.getByRole('dialog')
    await dialog.getByTestId('provider-kimi_code').click()

    await expect(dialog.getByTestId('kimi-code-start-oauth')).toBeVisible()
    await expect(dialog.getByTestId('channel-base-url-input')).toHaveCount(0)
    await expect(dialog.getByTestId('channel-api-key-input')).toHaveCount(0)
  })
})
