'use client';

import { useState } from 'react';
import { IconPlayerPlay } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import type { Variable, DisplayField, VariableInput, DisplayFieldInput, TestResult } from '../data/schema';
import { useTestUsageMonitorChannel } from '../data/usage-monitor';
import { ParsedFieldDisplay } from './parsed-field-display';

interface Props {
  apiUrl: string;
  apiMethod: string;
  apiHeaders: string;
  apiBody: string;
  variables: Variable[];
  displayFields: DisplayField[];
  providerType?: string;
  apiKey?: string;
}

export function TestConnection({ apiUrl, apiMethod, apiHeaders, apiBody, variables, displayFields, providerType, apiKey }: Props) {
  const { t } = useTranslation();
  const testMutation = useTestUsageMonitorChannel();
  const [result, setResult] = useState<TestResult | null>(null);

  async function handleTest() {
    setResult(null);
    try {
      const variableInputs: VariableInput[] = variables.map((v) => ({
        key: v.key,
        path: v.path,
        type: v.type,
        groupIndex: v.groupIndex,
      }));
      const displayFieldInputs: DisplayFieldInput[] = displayFields.map((df) => ({
        key: df.key,
        label: df.label,
        valueRef: df.valueRef,
        format: df.format,
        unit: df.unit,
        totalRef: df.totalRef,
        displayOrder: df.displayOrder,
        badge: df.badge,
        badgePresets: df.badgePresets,
      }));

      const res = await testMutation.mutateAsync({
        apiUrl,
        apiMethod: apiMethod as 'GET' | 'POST',
        apiHeaders,
        apiBody: apiBody || undefined,
        providerType: providerType || undefined,
        apiKey: apiKey || undefined,
        variables: variableInputs,
        displayFields: displayFieldInputs,
      });
      setResult(res);
    } catch {
      // error handled by mutation hook
    }
  }

  return (
    <div className='space-y-3'>
      <Button type='button' variant='outline' onClick={handleTest} disabled={!apiUrl || testMutation.isPending}>
        <IconPlayerPlay className='mr-1.5 size-4' />
        {testMutation.isPending ? t('usageMonitor.testConnection') + '...' : t('usageMonitor.testConnection')}
      </Button>

      {result && (
        <div className='space-y-2'>
          {result.success ? (
            <div className='rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950'>
              <div className='text-sm font-medium text-green-800 dark:text-green-200'>{t('usageMonitor.testSuccess')}</div>

              {/* Extracted Variables */}
              {result.parsedFields && result.parsedFields.length > 0 && (
                <div className='mt-3 space-y-1.5'>
                  {result.parsedFields.map((f) => (
                    <div key={f.key}>
                      {f.error ? (
                        <div className='text-xs'>
                          <span className='font-medium text-green-700 dark:text-green-300'>{f.label}:</span>{' '}
                          <span className='text-red-600'>{f.error}</span>
                        </div>
                      ) : (
                        <ParsedFieldDisplay field={f} displayFields={displayFields} />
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div className='rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-950'>
              <div className='text-sm font-medium text-red-800 dark:text-red-200'>{t('usageMonitor.testFailed')}</div>
              {result.error && <div className='mt-1 text-xs text-red-700 dark:text-red-300'>{result.error}</div>}
            </div>
          )}

          {result.rawResponse && (
            <details className='group'>
              <summary className='text-muted-foreground hover:text-foreground cursor-pointer text-xs'>{t('usageMonitor.preview')}</summary>
              <pre className='bg-muted mt-1 max-h-48 overflow-auto rounded p-2 font-mono text-xs'>{result.rawResponse}</pre>
            </details>
          )}
        </div>
      )}
    </div>
  );
}
