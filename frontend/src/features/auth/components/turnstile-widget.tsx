import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import type { TurnstileWidgetHandle } from './turnstile-widget.types';

const TURNSTILE_SCRIPT_ID = 'cloudflare-turnstile-script';
const TURNSTILE_SCRIPT_URL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';

let turnstileScriptPromise: Promise<TurnstileAPI> | null = null;
let turnstileScriptElement: HTMLScriptElement | null = null;

interface TurnstileRenderOptions {
  sitekey: string;
  action: string;
  language: 'en' | 'zh-cn';
  size: 'flexible';
  callback: (token: string) => void;
  'expired-callback': () => void;
  'timeout-callback': () => void;
  'error-callback': () => void;
}

interface TurnstileAPI {
  render(container: HTMLElement, options: TurnstileRenderOptions): string;
  reset(widgetId: string): void;
  remove(widgetId: string): void;
}

declare global {
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

function loadTurnstileScript(): Promise<TurnstileAPI> {
  if (window.turnstile) {
    return Promise.resolve(window.turnstile);
  }

  if (turnstileScriptPromise) {
    return turnstileScriptPromise;
  }

  turnstileScriptPromise = new Promise<TurnstileAPI>((resolve, reject) => {
    const existingScript = document.getElementById(TURNSTILE_SCRIPT_ID) as HTMLScriptElement | null;
    if (existingScript?.dataset.loaded === 'true') {
      existingScript.remove();
      turnstileScriptElement = null;
      turnstileScriptPromise = null;
      reject(new Error('Cloudflare Turnstile did not initialize'));
      return;
    }

    const script = existingScript ?? document.createElement('script');
    turnstileScriptElement = script;

    const handleLoad = () => {
      script.dataset.loaded = 'true';
      if (window.turnstile) {
        resolve(window.turnstile);
        return;
      }

      turnstileScriptPromise = null;
      reject(new Error('Cloudflare Turnstile did not initialize'));
    };
    const handleError = () => {
      script.remove();
      turnstileScriptElement = null;
      turnstileScriptPromise = null;
      reject(new Error('Failed to load Cloudflare Turnstile'));
    };

    script.addEventListener('load', handleLoad, { once: true });
    script.addEventListener('error', handleError, { once: true });

    if (!existingScript) {
      script.id = TURNSTILE_SCRIPT_ID;
      script.src = TURNSTILE_SCRIPT_URL;
      script.async = true;
      script.defer = true;
      document.head.appendChild(script);
    }
  });

  return turnstileScriptPromise;
}

function retryTurnstileScript() {
  if (!window.turnstile) {
    turnstileScriptElement?.remove();
    turnstileScriptElement = null;
    turnstileScriptPromise = null;
  }
}

interface TurnstileWidgetProps {
  siteKey: string;
  action: string;
  language: string;
  testId: string;
  onTokenChange: (token: string | null) => void;
  onError: () => void;
  onExpired: () => void;
}

export const TurnstileWidget = forwardRef<TurnstileWidgetHandle, TurnstileWidgetProps>(function TurnstileWidget(
  { siteKey, action, language, testId, onTokenChange, onError, onExpired },
  ref
) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetRef = useRef<{ api: TurnstileAPI; id: string } | null>(null);
  const [renderAttempt, setRenderAttempt] = useState(0);
  const callbackRef = useRef({ onTokenChange, onError, onExpired });

  callbackRef.current = { onTokenChange, onError, onExpired };

  useImperativeHandle(
    ref,
    () => ({
      reset(options) {
        callbackRef.current.onTokenChange(null);
        if (options?.reloadScript) {
          retryTurnstileScript();
          setRenderAttempt((attempt) => attempt + 1);
          return;
        }
        if (widgetRef.current) {
          widgetRef.current.api.reset(widgetRef.current.id);
        }
      },
    }),
    []
  );

  useEffect(() => {
    let cancelled = false;

    callbackRef.current.onTokenChange(null);
    if (widgetRef.current) {
      widgetRef.current.api.remove(widgetRef.current.id);
      widgetRef.current = null;
    }
    loadTurnstileScript()
      .then((api) => {
        if (cancelled || !containerRef.current) {
          return;
        }

        const id = api.render(containerRef.current, {
          sitekey: siteKey,
          action,
          language: language.toLowerCase().startsWith('zh') ? 'zh-cn' : 'en',
          size: 'flexible',
          callback: (token) => callbackRef.current.onTokenChange(token),
          'expired-callback': () => {
            callbackRef.current.onTokenChange(null);
            callbackRef.current.onExpired();
          },
          'timeout-callback': () => {
            callbackRef.current.onTokenChange(null);
            callbackRef.current.onExpired();
          },
          'error-callback': () => {
            callbackRef.current.onTokenChange(null);
            callbackRef.current.onError();
          },
        });
        widgetRef.current = { api, id };
      })
      .catch(() => {
        if (!cancelled) {
          callbackRef.current.onTokenChange(null);
          callbackRef.current.onError();
        }
      });

    return () => {
      cancelled = true;
      if (widgetRef.current) {
        widgetRef.current.api.remove(widgetRef.current.id);
        widgetRef.current = null;
      }
    };
  }, [action, language, renderAttempt, siteKey]);

  return <div ref={containerRef} className='w-full min-w-0' data-testid={testId} />;
});
