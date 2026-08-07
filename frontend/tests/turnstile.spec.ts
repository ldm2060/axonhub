import { expect, Page, Route, test } from '@playwright/test'

const TURNSTILE_SCRIPT_URL = '**/challenges.cloudflare.com/turnstile/v0/api.js*'

interface TurnstileWidgetState {
  action: string
  renderCount: number
  resetCount: number
}

async function mockCommonPublicRoutes(page: Page) {
  await page.route('**/admin/system/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ isInitialized: true }),
    })
  })

  await page.route('**/auth/signup/allowed', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ allowed: true }),
    })
  })

  await page.route('**/oauth/oidc/providers', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [] }),
    })
  })
}

async function mockAuthConfig(page: Page, enabled: boolean) {
  await page.route('**/auth/config', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        turnstile: {
          enabled,
          site_key: enabled ? 'test-site-key' : '',
          actions: {
            signin: 'signin',
            signup_send_code: 'signup_send_code',
            signup: 'signup',
          },
        },
      }),
    })
  })
}

async function installTurnstileMock(page: Page) {
  await page.route(TURNSTILE_SCRIPT_URL, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/javascript',
      body: `
        (() => {
          let nextWidgetID = 1;
          const widgets = new Map();
          const publicState = {};
          window.__turnstileTest = publicState;
          window.turnstile = {
            render(container, options) {
              const id = 'turnstile-widget-' + nextWidgetID++;
              widgets.set(id, { container, options });
              const state = publicState[options.action] || {
                action: options.action,
                renderCount: 0,
                resetCount: 0,
              };
              state.renderCount += 1;
              publicState[options.action] = state;
              container.dataset.widgetId = id;
              container.dataset.action = options.action;
              container.style.minHeight = '65px';
              return id;
            },
            reset(id) {
              const widget = widgets.get(id);
              if (!widget) return;
              const state = publicState[widget.options.action];
              state.resetCount += 1;
              widget.container.dataset.resetCount = String(state.resetCount);
            },
            remove(id) {
              const widget = widgets.get(id);
              if (!widget) return;
              delete widget.container.dataset.widgetId;
              widgets.delete(id);
            },
          };
          publicState.solve = (action, token) => {
            for (const widget of widgets.values()) {
              if (widget.options.action === action) {
                widget.options.callback(token);
                return true;
              }
            }
            return false;
          };
          publicState.expire = (action) => {
            for (const widget of widgets.values()) {
              if (widget.options.action === action) {
                widget.options['expired-callback']();
                return true;
              }
            }
            return false;
          };
          publicState.fail = (action) => {
            for (const widget of widgets.values()) {
              if (widget.options.action === action) {
                widget.options['error-callback']();
                return true;
              }
            }
            return false;
          };
        })();
      `,
    })
  })
}

async function solveTurnstile(page: Page, action: string, token: string) {
  const solved = await page.evaluate(
    ([challengeAction, challengeToken]) => {
      const turnstileTest = (
        window as typeof window & {
          __turnstileTest?: {
            solve?: (action: string, token: string) => boolean
          }
        }
      ).__turnstileTest
      return turnstileTest?.solve?.(challengeAction, challengeToken) === true
    },
    [action, token]
  )
  expect(solved).toBe(true)
}

async function getTurnstileState(page: Page, action: string): Promise<TurnstileWidgetState | null> {
  return page.evaluate((challengeAction) => {
    const turnstileTest = (
      window as typeof window & {
        __turnstileTest?: Record<string, TurnstileWidgetState>
      }
    ).__turnstileTest
    return turnstileTest?.[challengeAction] ?? null
  }, action)
}

async function expectJSONBody(route: Route) {
  const body = route.request().postDataJSON()
  expect(body).toEqual(expect.any(Object))
  return body as Record<string, unknown>
}

test.describe('Cloudflare Turnstile authentication', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.removeItem('axonhub_access_token')
    })
    await mockCommonPublicRoutes(page)
  })

  test('does not render widgets when Turnstile is disabled', async ({ page }) => {
    await mockAuthConfig(page, false)

    await page.goto('/sign-in')
    await expect(page.getByTestId('sign-in-email')).toBeVisible()
    await expect(page.getByTestId('turnstile-signin')).toHaveCount(0)

    await page.goto('/sign-up')
    await expect(page.getByTestId('sign-up-email')).toBeVisible()
    await expect(page.getByTestId('turnstile-signup-send-code')).toHaveCount(0)
    await expect(page.getByTestId('turnstile-signup')).toHaveCount(0)
  })

  test('sends and resets the password sign-in token', async ({ page }) => {
    await mockAuthConfig(page, true)
    await installTurnstileMock(page)

    let signInBody: Record<string, unknown> | null = null
    await page.route('**/auth/signin', async (route) => {
      signInBody = await expectJSONBody(route)
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Invalid credentials' }),
      })
    })

    await page.goto('/sign-in')
    await expect(page.getByTestId('turnstile-signin')).toHaveAttribute('data-action', 'signin')
    await solveTurnstile(page, 'signin', 'signin-token')

    await page.getByTestId('sign-in-email').fill('my@example.com')
    await page.getByTestId('sign-in-password').fill('pwd123456')
    await page.getByTestId('sign-in-submit').click()

    await expect.poll(() => signInBody).not.toBeNull()
    expect(signInBody).toMatchObject({
      email: 'my@example.com',
      password: 'pwd123456',
      turnstile_token: 'signin-token',
    })
    await expect.poll(async () => (await getTurnstileState(page, 'signin'))?.resetCount).toBe(1)

    await page.getByTestId('sign-in-submit').click()
    await expect(page.getByText(/complete the security check|完成安全验证/i)).toBeVisible()
  })

  test('keeps send-code and signup challenges independent', async ({ page }) => {
    await mockAuthConfig(page, true)
    await installTurnstileMock(page)

    const sendCodeBodies: Record<string, unknown>[] = []
    await page.route('**/auth/signup/verification-code', async (route) => {
      sendCodeBodies.push(await expectJSONBody(route))
      await route.fulfill({
        status: sendCodeBodies.length === 1 ? 400 : 200,
        contentType: 'application/json',
        body: JSON.stringify(
          sendCodeBodies.length === 1
            ? { message: 'Unable to send verification code' }
            : { message: 'Verification code sent' }
        ),
      })
    })

    let signUpBody: Record<string, unknown> | null = null
    await page.route('**/auth/signup', async (route) => {
      signUpBody = await expectJSONBody(route)
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Unable to create account' }),
      })
    })

    await page.goto('/sign-up')
    await expect(page.getByTestId('turnstile-signup-send-code')).toHaveAttribute('data-action', 'signup_send_code')
    await expect(page.getByTestId('turnstile-signup')).toHaveAttribute('data-action', 'signup')

    await page.getByTestId('sign-up-email').fill('my@example.com')
    await solveTurnstile(page, 'signup_send_code', 'send-code-token-1')
    await page.getByTestId('sign-up-send-code').click()
    await expect.poll(() => sendCodeBodies.length).toBe(1)
    expect(sendCodeBodies[0]).toMatchObject({
      email: 'my@example.com',
      turnstile_token: 'send-code-token-1',
    })
    await expect.poll(async () => (await getTurnstileState(page, 'signup_send_code'))?.resetCount).toBe(1)
    await expect.poll(async () => (await getTurnstileState(page, 'signup'))?.resetCount).toBe(0)

    await page.getByTestId('sign-up-send-code').click()
    await expect(page.getByText(/complete the security check|完成安全验证/i).first()).toBeVisible()
    expect(sendCodeBodies).toHaveLength(1)

    await solveTurnstile(page, 'signup_send_code', 'send-code-token-2')
    await page.getByTestId('sign-up-send-code').click()
    await expect.poll(() => sendCodeBodies.length).toBe(2)
    expect(sendCodeBodies[1]).toMatchObject({ turnstile_token: 'send-code-token-2' })

    await page.getByTestId('sign-up-first-name').fill('Test')
    await page.getByTestId('sign-up-last-name').fill('User')
    await page.getByTestId('sign-up-verification-code').fill('123456')
    await page.getByTestId('sign-up-password').fill('pwd123456')
    await page.getByTestId('sign-up-confirm-password').fill('pwd123456')
    await solveTurnstile(page, 'signup', 'signup-token')
    await page.getByTestId('sign-up-submit').click()

    await expect.poll(() => signUpBody).not.toBeNull()
    expect(signUpBody).toMatchObject({
      email: 'my@example.com',
      first_name: 'Test',
      last_name: 'User',
      verification_code: '123456',
      turnstile_token: 'signup-token',
    })
    await expect.poll(async () => (await getTurnstileState(page, 'signup'))?.resetCount).toBe(1)
    await expect(page.getByTestId('sign-up-email')).toHaveValue('my@example.com')
  })

  test('clears expired and errored tokens before requests', async ({ page }) => {
    await mockAuthConfig(page, true)
    await installTurnstileMock(page)

    let requestCount = 0
    await page.route('**/auth/signin', async (route) => {
      requestCount += 1
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Invalid credentials' }),
      })
    })

    await page.goto('/sign-in')
    await page.getByTestId('sign-in-email').fill('my@example.com')
    await page.getByTestId('sign-in-password').fill('pwd123456')
    await solveTurnstile(page, 'signin', 'expired-token')

    const expired = await page.evaluate(() => {
      const turnstileTest = (
        window as typeof window & {
          __turnstileTest?: { expire?: (action: string) => boolean }
        }
      ).__turnstileTest
      return turnstileTest?.expire?.('signin') === true
    })
    expect(expired).toBe(true)
    await expect(page.getByText(/security check expired|安全验证已过期/i)).toBeVisible()
    await page.getByTestId('sign-in-submit').click()
    await expect(page.getByText(/complete the security check|完成安全验证/i)).toBeVisible()
    expect(requestCount).toBe(0)

    await solveTurnstile(page, 'signin', 'errored-token')
    const failed = await page.evaluate(() => {
      const turnstileTest = (
        window as typeof window & {
          __turnstileTest?: { fail?: (action: string) => boolean }
        }
      ).__turnstileTest
      return turnstileTest?.fail?.('signin') === true
    })
    expect(failed).toBe(true)
    await expect(page.getByText(/could not be completed|无法完成/i)).toBeVisible()
    await page.getByTestId('sign-in-submit').click()
    await expect(page.getByText(/complete the security check|完成安全验证/i)).toBeVisible()
    expect(requestCount).toBe(0)
  })

  test('blocks password operations when config fails without blocking OIDC', async ({ page }) => {
    await page.route('**/auth/config', async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Authentication security configuration is unavailable' }),
      })
    })
    await page.route('**/oauth/oidc/providers', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'test-oidc',
              name: 'test-oidc',
              display_name: 'Test SSO',
              jit_enabled: false,
              icon_url: '',
              button_color: '',
              active: true,
              oidc_login_only: false,
              is_linked: false,
            },
          ],
        }),
      })
    })

    await page.goto('/sign-in')
    await expect(page.getByTestId('sign-in-submit')).toBeDisabled()
    await expect(page.getByRole('button', { name: /Test SSO/i })).toBeEnabled()
    await expect(page.getByText(/security configuration could not be loaded|无法加载安全配置/i)).toBeVisible()
  })

  test('keeps both signup widgets inside a narrow viewport', async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 740 })
    await mockAuthConfig(page, true)
    await installTurnstileMock(page)

    await page.goto('/sign-up')
    for (const testID of ['turnstile-signup-send-code', 'turnstile-signup']) {
      const widget = page.getByTestId(testID)
      await expect(widget).toBeVisible()
      const box = await widget.boundingBox()
      expect(box).not.toBeNull()
      expect(box!.x).toBeGreaterThanOrEqual(0)
      expect(box!.x + box!.width).toBeLessThanOrEqual(320)
    }
  })
})
