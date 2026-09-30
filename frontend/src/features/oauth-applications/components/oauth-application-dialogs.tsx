'use client';

import { CreateOAuthApplicationDialog } from './create-oauth-application-dialog';
import { DeleteOAuthApplicationDialog } from './delete-oauth-application-dialog';
import { EditOAuthApplicationDialog } from './edit-oauth-application-dialog';
import { RotateSecretDialog } from './rotate-secret-dialog';

export function OAuthApplicationDialogs() {
  return (
    <>
      <CreateOAuthApplicationDialog />
      <EditOAuthApplicationDialog />
      <RotateSecretDialog />
      <DeleteOAuthApplicationDialog />
    </>
  );
}
