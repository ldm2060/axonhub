# Email Verification Code Registration Design

## Summary

Replace link-based registration email verification with a registration-time 6-digit email code. Users request a code from the sign-up form, enter it before submitting registration, and the backend creates the account only after the code is valid.

This avoids the current broken experience where verification links always report invalid or expired. Link verification pages and other frontend pages that only exist for that old flow will be removed.

## Goals

- Let users complete email verification directly in the registration form.
- Allow users to resend a verification code when the previous code expires or was not received.
- Preserve the existing administrator approval setting after email verification succeeds.
- Stop depending on `/verify-email` result pages or email verification links for new registration.

## Non-goals

- Do not redesign password reset; password reset can continue using token links.
- Do not add new registration settings.
- Do not add a separate email verification code table for this change.
- Do not create a user before the email code is validated.

## Backend Design

### Token model

The current `EmailToken` model is user-owned through a required `user_id`, but registration codes must be sent before a user exists. Keep using the same token infrastructure, but extend it for pre-registration codes:

- Add a nullable `email` field that stores the normalized email for pre-registration verification codes.
- Make `user_id` optional/nillable so `verify_email` codes can exist before account creation.
- Existing user-owned tokens, including password reset tokens, continue to use `user_id`.
- Registration verification code lookup uses `type = verify_email`, normalized `email`, `token`, `expires_at`, and `consumed_at`.
- Add indexes needed for email/code lookup and cleanup, then regenerate Ent code.

### Verification code endpoint

Add `POST /auth/signup/verification-code`.

Request:

```json
{
  "email": "user@example.com"
}
```

Behavior:

1. Confirm self-registration is enabled.
2. Validate the email format and existing allow/deny email pattern rules.
3. Reject already registered email addresses with the same style of error currently used by sign-up.
4. Generate a 6-digit numeric code.
5. Store the code using the existing `EmailToken` infrastructure after extending it for pre-registration email ownership:
   - `type = verify_email`
   - `token = <six digit code>`
   - `email = <normalized email>`
   - `user_id = null` until a user exists
   - `expires_at = now + 5 minutes`
6. Retry code generation on the rare case where the 6-digit code collides with the existing unique token constraint.
7. Send an email containing only the code and its expiration time.

The resend rate limit remains one send per email per minute. Requesting a new code invalidates or supersedes older unconsumed verification codes for that email by consuming existing `verify_email` tokens with the same normalized email before storing the new code.

### Sign-up endpoint

Extend `POST /auth/signup` with a required `verificationCode` field.

New behavior:

1. Validate sign-up input as today.
2. Validate the email/code pair before creating the user.
3. If the code is missing, malformed, expired, consumed, or not the latest valid code for that email, return: `验证码无效或已过期，请重新获取`.
4. Consume the code after successful validation.
5. Create the user.
6. If administrator approval is enabled, keep the user `pending`.
7. If administrator approval is disabled, activate the user immediately with default user scopes.

The old behavior that creates a pending user before email verification is removed from the registration flow.

### Legacy link flow

New registration emails will not contain verification links. The frontend `/verify-email` pages and routes will be removed. Backend link verification can either be removed if unused by the server route table, or left unreachable only if required to avoid breaking unrelated code during implementation. It must not remain part of the user-facing registration path.

## Frontend Design

### Sign-up form

Update the sign-up form to include:

- Email input.
- `发送验证码` button next to the email input.
- 60-second resend countdown after a successful send.
- 6-digit numeric verification code input.
- Existing name and password fields.

Submitting the form requires a valid-looking 6-digit code. Server-side validation remains authoritative.

### User states

- Code sent: show `验证码已发送，请查收邮箱`.
- Code invalid or expired: show `验证码无效或已过期，请重新获取` near the code field.
- Registration succeeds with administrator approval enabled: show `邮箱已验证，账号等待管理员审批` in the sign-up form result area.
- Registration succeeds without administrator approval: show `注册成功，请登录` in the sign-up form result area.

### Removed pages

Delete frontend pages, routes, and i18n strings that exist only for the old email verification result page flow, including the `/verify-email` success/failure/pending page. Registration no longer navigates to a separate verification result page.

## Email Design

Replace the verification email template content with a code-based message:

- Include the 6-digit verification code prominently.
- State that the code expires in 5 minutes.
- Do not include a verification link.
- Keep both HTML and text templates.

## Error Handling and Security

- Keep the resend rate limit at one request per email per minute.
- Do not create a user until the email code has been validated.
- Treat expired, consumed, malformed, and incorrect codes as the same user-facing error.
- Consume the code only after validation succeeds and before user creation completes.
- Continue using the existing email allow/deny rules before sending a code.
- Avoid keeping the broken link flow visible to users.

## Testing

Backend tests should cover:

- Sending a code for an allowed new email.
- Rejecting code sends when sign-up is disabled.
- Rejecting code sends for disallowed email patterns.
- Rejecting sign-up with missing, malformed, expired, consumed, or wrong codes.
- Creating a pending user when approval is required.
- Creating an activated user when approval is not required.
- Ensuring no user is created when code validation fails.

Frontend checks should cover:

- Send-code button behavior and countdown.
- 6-digit code validation.
- Successful pending-approval result.
- Successful activated result.
- Invalid/expired code error display.
- Removal of the old verify-email route/page from the router.

## Implementation Notes

Use the existing `EmailToken` infrastructure rather than adding a new table, but extend the schema so pre-registration verification codes can be owned by normalized email before a user exists. Add a service method such as `ValidateEmailCode(ctx, email, code)` for email/code lookup, and run code consumption plus user creation in one transaction so failed registration cannot leave a consumed code without an account.
