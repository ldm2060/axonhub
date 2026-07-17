import { apiRequest } from '@/lib/api-client';

export interface KimiCodeDeviceFlowStartResult {
  session_id: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete: string;
  expires_in: number;
  interval: number;
}

export interface KimiCodeModel {
  id: string;
  context_length: number;
  protocol?: 'kimi' | 'anthropic';
  supports_reasoning?: boolean;
  supports_image_in?: boolean;
  supports_video_in?: boolean;
  supports_tool_use?: boolean;
  supports_thinking_type?: 'only' | 'no' | 'both';
}

export interface KimiCodeDeviceFlowPollResult {
  status: 'pending' | 'slow_down' | 'complete';
  message?: string;
  credentials?: string;
  models?: KimiCodeModel[];
}

export async function kimiCodeOAuthStart(): Promise<KimiCodeDeviceFlowStartResult> {
  return apiRequest('/admin/kimicode/oauth/start', { method: 'POST', body: {}, requireAuth: true });
}

export async function kimiCodeOAuthPoll(input: { session_id: string }): Promise<KimiCodeDeviceFlowPollResult> {
  return apiRequest('/admin/kimicode/oauth/poll', { method: 'POST', body: input, requireAuth: true });
}
