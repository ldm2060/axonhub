import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

export interface DeviceFlowStartResult {
  session_id: string;
  user_code: string;
  verification_uri: string;
  verification_uri_complete?: string;
  expires_in: number;
  interval: number;
}

export interface DeviceFlowPollResult<T> {
  status?: 'pending' | 'slow_down' | 'complete';
  message?: string;
  completion?: T;
}

interface UseDeviceFlowOptions<T> {
  start: () => Promise<DeviceFlowStartResult>;
  poll: (input: { session_id: string }) => Promise<DeviceFlowPollResult<T>>;
  onSuccess?: (completion: T) => void;
}

export function useDeviceFlow<T>({ start: startRequest, poll: pollRequest, onSuccess }: UseDeviceFlowOptions<T>) {
  const { t } = useTranslation();
  const [userCode, setUserCode] = useState<string | null>(null);
  const [verificationUri, setVerificationUri] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [interval, setInterval] = useState(5);
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isComplete, setIsComplete] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef = useRef(5);
  const onSuccessRef = useRef(onSuccess);

  useEffect(() => { onSuccessRef.current = onSuccess; }, [onSuccess]);
  useEffect(() => () => { if (timeoutRef.current) clearTimeout(timeoutRef.current); }, []);

  const reset = useCallback(() => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    timeoutRef.current = null;
    setUserCode(null); setVerificationUri(null); setSessionId(null); setExpiresAt(null); setInterval(5);
    intervalRef.current = 5; setIsPolling(false); setError(null); setIsComplete(false);
  }, []);

  const poll = useCallback(async (activeSession: string, expiry: number) => {
    if (Date.now() >= expiry) { setIsPolling(false); setError(t('channels.dialogs.oauth.errors.deviceFlowExpired')); return; }
    try {
      const result = await pollRequest({ session_id: activeSession });
      if (result.status === 'complete' && result.completion !== undefined) {
        setIsPolling(false); setIsComplete(true); onSuccessRef.current?.(result.completion);
        toast.success(t('channels.dialogs.oauth.messages.credentialsImported'));
        return;
      }
      if (result.status === 'slow_down') { intervalRef.current *= 2; setInterval(intervalRef.current); }
      if (result.status === 'pending' || result.status === 'slow_down') {
        timeoutRef.current = window.setTimeout(() => { void poll(activeSession, expiry); }, intervalRef.current * 1000);
        return;
      }
      setIsPolling(false); setError(result.message || 'Device authorization failed');
    } catch (cause) { setIsPolling(false); setError(cause instanceof Error ? cause.message : String(cause)); }
  }, [pollRequest, t]);

  const start = useCallback(async () => {
    reset(); setIsPolling(true);
    try {
      const result = await startRequest();
      const expiry = Date.now() + result.expires_in * 1000;
      setUserCode(result.user_code); setVerificationUri(result.verification_uri_complete || result.verification_uri);
      setSessionId(result.session_id); setExpiresAt(expiry); setInterval(result.interval); intervalRef.current = result.interval;
      void poll(result.session_id, expiry);
    } catch (cause) { setIsPolling(false); setError(cause instanceof Error ? cause.message : String(cause)); }
  }, [poll, reset, startRequest]);

  return { userCode, verificationUri, sessionId, expiresAt, interval, isPolling, error, isComplete, start, reset };
}
