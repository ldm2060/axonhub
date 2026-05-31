'use client';

import { useState } from 'react';
import { IconPlayerPlay } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { useTestUsageMonitorChannel } from '../data/usage-monitor';
import type { FieldConfig, TestResult } from '../data/schema';

interface Props {
  apiUrl: string;
  apiMethod: string;
  apiHeaders: string;
  apiBody: string;
  fields: FieldConfig[];
}

export function TestConnection({ apiUrl, apiMethod, apiHeaders, apiBody, fields }: Props) {
  const { t } = useTranslation();
  const testMutation = useTestUsageMonitorChannel();
  const [result, setResult] = useState<TestResult | null>(null);

  async function handleTest() {
    setResult(null);
    try {
      const res = await testMutation.mutateAsync({
        apiUrl,
        apiMethod: apiMethod as 'GET' | 'POST',
        apiHeaders,
        apiBody: apiBody || undefined,
        fields,
      });
      setResult(res);
    } catch {
      // error handled by mutation hook
    }
  }

  return (
    <div className="space-y-3">
      <Button
        type="button"
        variant="outline"
        onClick={handleTest}
        disabled={!apiUrl || testMutation.isPending}
      >
        <IconPlayerPlay className="mr-1.5 size-4" />
        {testMutation.isPending ? t('usageMonitor.testConnection') + '...' : t('usageMonitor.testConnection')}
      </Button>

      {result && (
        <div className="space-y-2">
          {result.success ? (
            <div className="rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950">
              <div className="text-sm font-medium text-green-800 dark:text-green-200">
                {t('usageMonitor.testSuccess')}
              </div>
              {result.parsedFields && result.parsedFields.length > 0 && (
                <div className="mt-2 space-y-1">
                  {result.parsedFields.map((f) => (
                    <div key={f.key} className="text-xs text-green-700 dark:text-green-300">
                      <span className="font-medium">{f.label}:</span>{' '}
                      {f.error ? (
                        <span className="text-red-600">{f.error}</span>
                      ) : (
                        <span>
                          {f.value !== null ? String(f.value) : '-'}
                          {f.unit ? ` ${f.unit}` : ''}
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div className="rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-950">
              <div className="text-sm font-medium text-red-800 dark:text-red-200">
                {t('usageMonitor.testFailed')}
              </div>
              {result.error && (
                <div className="mt-1 text-xs text-red-700 dark:text-red-300">{result.error}</div>
              )}
            </div>
          )}

          {result.rawResponse && (
            <details className="group">
              <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                {t('usageMonitor.preview')}
              </summary>
              <pre className="mt-1 max-h-48 overflow-auto rounded bg-muted p-2 text-xs font-mono">
                {result.rawResponse}
              </pre>
            </details>
          )}
        </div>
      )}
    </div>
  );
}
