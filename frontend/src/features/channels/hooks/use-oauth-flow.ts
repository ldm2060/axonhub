import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { ProxyType, type ProxyConfig } from '../data/schema';

export interface OAuthStartResult {
  session_id: string;
  auth_url: string;
}

export interface OAuthExchangeInput {
  session_id: string;
  callback_url?: string;
  proxy?: ProxyConfig;
}

export interface OAuthExchangeResult {
  credentials: string;
}

export interface OAuthFlowOptions {
  /**
   * Function to call the OAuth start endpoint
   */
  startFn: (headers?: Record<string, string>) => Promise<OAuthStartResult>;

  /**
   * Function to call the OAuth exchange endpoint
   */
  exchangeFn: (input: OAuthExchangeInput, headers?: Record<string, string>) => Promise<OAuthExchangeResult>;

  /**
   * Optional proxy configuration for exchange token request
   */
  proxyConfig?: ProxyConfig;

  /**
   * Callback when credentials are successfully obtained
   */
  onSuccess?: (credentials: string) => void;

  /**
   * When true, the flow needs no pasted callback URL — exchange polls the
   * backend until the browser login completes (e.g. ZCode cli flow).
   */
  pollMode?: boolean;
}

export interface OAuthFlowState {
  sessionId: string | null;
  authUrl: string | null;
  callbackUrl: string;
  isStarting: boolean;
  isExchanging: boolean;
  /** True when the flow polls server-side and needs no pasted callback. */
  pollMode: boolean;
}

export interface OAuthFlowActions {
  start: () => Promise<void>;
  exchange: () => Promise<void>;
  setCallbackUrl: (url: string) => void;
  reset: () => void;
}

/**
 * A reusable hook for managing OAuth flows (e.g., Codex, Claude Code).
 * This eliminates code duplication for different OAuth providers.
 *
 * @example
 * ```tsx
 * const codexOAuth = useOAuthFlow({
 *   startFn: codexOAuthStart,
 *   exchangeFn: codexOAuthExchange,
 *   onSuccess: (credentials) => form.setValue('credentials.apiKey', credentials),
 * });
 *
 * // Later in your component:
 * <Button onClick={codexOAuth.start} disabled={codexOAuth.isStarting}>
 *   {codexOAuth.isStarting ? 'Starting...' : 'Start OAuth'}
 * </Button>
 * ```
 */
export function useOAuthFlow(options: OAuthFlowOptions): OAuthFlowState & OAuthFlowActions {
  const { startFn, exchangeFn, proxyConfig, onSuccess, pollMode } = options;
  const { t } = useTranslation();

  const [sessionId, setSessionId] = useState<string | null>(null);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [callbackUrl, setCallbackUrl] = useState('');
  const [isStarting, setIsStarting] = useState(false);
  const [isExchanging, setIsExchanging] = useState(false);

  const runExchange = useCallback(
    async (sid: string, pastedCallbackUrl: string) => {
      if (!sid) {
        toast.error(t('channels.dialogs.oauth.errors.sessionMissing'));
        return;
      }

      if (!pollMode && !pastedCallbackUrl.trim()) {
        toast.error(t('channels.dialogs.oauth.errors.callbackUrlRequired'));
        return;
      }

      setIsExchanging(true);
      try {
        const exchangeInput: OAuthExchangeInput = {
          session_id: sid,
        };
        // Send the pasted callback URL whenever present — even in pollMode, where
        // it takes precedence as a fallback path (e.g. ZCode zcode:// callback).
        if (pastedCallbackUrl.trim()) {
          exchangeInput.callback_url = pastedCallbackUrl.trim();
        }

        // Add proxy config if provided and type is not disabled/environment
        if (proxyConfig && proxyConfig.type === ProxyType.URL) {
          exchangeInput.proxy = {
            type: proxyConfig.type,
            url: proxyConfig.url,
            ...(proxyConfig.username && { username: proxyConfig.username }),
            ...(proxyConfig.password && { password: proxyConfig.password }),
          };
        }

        const result = await exchangeFn(exchangeInput);

        if (onSuccess) {
          onSuccess(result.credentials);
        }

        toast.success(t('channels.dialogs.oauth.messages.credentialsImported'));
      } catch (error) {
        toast.error(error instanceof Error ? error.message : String(error));
      } finally {
        setIsExchanging(false);
      }
    },
    [pollMode, exchangeFn, onSuccess, t, proxyConfig]
  );

  const exchange = useCallback(() => runExchange(sessionId ?? '', callbackUrl), [runExchange, sessionId, callbackUrl]);

  const start = useCallback(async () => {
    setIsStarting(true);
    try {
      const result = await startFn();
      setSessionId(result.session_id);
      setAuthUrl(result.auth_url);
      // Mirror the desktop client: once the authorize page is up, poll for the
      // finished login in the background — credentials auto-fill when it
      // completes, no button click needed.
      if (pollMode) {
        void runExchange(result.session_id, '');
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsStarting(false);
    }
  }, [startFn, pollMode, runExchange]);

  const reset = useCallback(() => {
    setSessionId(null);
    setAuthUrl(null);
    setCallbackUrl('');
    setIsStarting(false);
    setIsExchanging(false);
  }, []);

  return {
    sessionId,
    authUrl,
    callbackUrl,
    isStarting,
    isExchanging,
    pollMode: !!pollMode,
    start,
    exchange,
    setCallbackUrl,
    reset,
  };
}
