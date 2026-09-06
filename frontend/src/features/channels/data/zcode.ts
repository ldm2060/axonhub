import { apiRequest } from '@/lib/api-client';
import type { ProxyConfig } from './schema';

export async function zcodeOAuthStart(headers?: Record<string, string>): Promise<{ session_id: string; auth_url: string }> {
  return apiRequest('/admin/zcode/oauth/start', {
    method: 'POST',
    body: {},
    headers,
    requireAuth: true,
  });
}

export async function zcodeOAuthExchange(
  input: {
    session_id: string;
    callback_url: string;
    proxy?: ProxyConfig;
  },
  headers?: Record<string, string>
): Promise<{ credentials: string }> {
  return apiRequest('/admin/zcode/oauth/exchange', {
    method: 'POST',
    body: input,
    headers,
    requireAuth: true,
  });
}
