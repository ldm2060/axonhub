import { z } from 'zod';

export const oauthClientStatusSchema = z.enum(['enabled', 'disabled']);
export const oauthClientTypeSchema = z.enum(['confidential', 'public']);

export const oauthApplicationSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullable().optional(),
  clientID: z.string(),
  redirectUris: z.array(z.string()),
  clientType: oauthClientTypeSchema,
  status: oauthClientStatusSchema,
  lastUsedAt: z.string().nullable().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export type OAuthApplication = z.infer<typeof oauthApplicationSchema>;

export const oauthApplicationsConnectionSchema = z.object({
  edges: z.array(z.object({ node: oauthApplicationSchema })),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    hasPreviousPage: z.boolean(),
    startCursor: z.string().nullable().optional(),
    endCursor: z.string().nullable().optional(),
  }),
  totalCount: z.number(),
});

export type OAuthApplicationsConnection = z.infer<typeof oauthApplicationsConnectionSchema>;

export const createOAuthApplicationInputSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  redirectUris: z.array(z.string().min(1)).min(1),
  clientType: oauthClientTypeSchema,
});

export type CreateOAuthApplicationInput = z.infer<typeof createOAuthApplicationInputSchema>;

export const updateOAuthApplicationInputSchema = z.object({
  name: z.string().min(1).optional(),
  description: z.string().optional(),
  redirectUris: z.array(z.string().min(1)).min(1).optional(),
  status: oauthClientStatusSchema.optional(),
});

export type UpdateOAuthApplicationInput = z.infer<typeof updateOAuthApplicationInputSchema>;

export const oauthAuthorizationRequestSchema = z.object({
  clientId: z.string(),
  clientName: z.string(),
  clientDescription: z.string().nullable().optional(),
  redirectUriHost: z.string(),
  scopes: z.array(z.string()),
  userEmail: z.string(),
});

export type OAuthAuthorizationRequest = z.infer<typeof oauthAuthorizationRequestSchema>;
