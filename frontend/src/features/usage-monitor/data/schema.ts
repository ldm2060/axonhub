import { z } from 'zod';
import { quotaMonitorBindingConditionSchema, quotaMonitorBindingTriggerStatusSchema } from '@/features/channels/data/schema';

export const fieldConfigSchema = z.object({
  key: z.string(),
  label: z.string(),
  path: z.string().optional(),
  type: z.enum(['jsonpath', 'regex']).optional(),
  format: z.enum(['percentage', 'fraction', 'number', 'datetime', 'text']),
  totalPath: z.string().optional().nullable(),
  unit: z.string().optional().nullable(),
  groupIndex: z.array(z.number()).optional().nullable(),
  displayOrder: z.number(),
  expression: z.string().optional(),
});

export const variableSchema = z.object({
  key: z.string(),
  path: z.string(),
  type: z.enum(['jsonpath', 'regex']),
  groupIndex: z.array(z.number()).optional().nullable(),
});

export const displayFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  valueRef: z.string(),
  format: z.enum(['percentage', 'fraction', 'number', 'datetime', 'text']),
  unit: z.string().optional().nullable(),
  totalRef: z.string().optional().nullable(),
  displayOrder: z.number(),
  badge: z.string().optional().nullable(),
  badgePresets: z.string().optional().nullable(),
  group: z.string().optional().nullable(),
  groupLabelRef: z.string().optional().nullable(),
});

export const parsedFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  value: z.any().nullable(),
  total: z.any().nullable().optional(),
  percent: z.number().optional(),
  unit: z.string().optional(),
  format: z.string(),
  error: z.string().optional().nullable(),
  group: z.string().optional().nullable(),
  groupLabel: z.string().optional().nullable(),
});

export const usageMonitorChannelSchema = z.object({
  id: z.string(),
  name: z.string(),
  source: z.enum(['builtin', 'custom', 'template']),
  providerType: z.string().nullable().optional(),
  apiKey: z.string().nullable().optional(),
  channel: z
    .object({
      id: z.string(),
      name: z.string(),
      type: z.string(),
    })
    .nullable()
    .optional(),
  apiUrl: z.string(),
  apiMethod: z.enum(['GET', 'POST']).nullable(),
  apiHeaders: z.string(),
  apiBody: z.string().nullable().optional(),
  pollInterval: z.number(),
  fields: z.array(fieldConfigSchema),
  variables: z.array(variableSchema),
  displayFields: z.array(displayFieldSchema),
  status: z.enum(['active', 'paused', 'error']),
  lastPollAt: z.string().nullable().optional(),
  parsedData: z.array(parsedFieldSchema).nullable().optional(),
  lastPollError: z.string().nullable().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const testResultSchema = z.object({
  success: z.boolean(),
  rawResponse: z.string().optional(),
  parsedFields: z.array(parsedFieldSchema).optional(),
  error: z.string().optional(),
});

export type FieldConfig = z.infer<typeof fieldConfigSchema>;
export type Variable = z.infer<typeof variableSchema>;
export type DisplayField = z.infer<typeof displayFieldSchema>;
export type ParsedField = z.infer<typeof parsedFieldSchema>;
export type UsageMonitorChannel = z.infer<typeof usageMonitorChannelSchema>;
export type TestResult = z.infer<typeof testResultSchema>;

export interface VariableInput {
  key: string;
  path: string;
  type: string;
  groupIndex?: number[];
}

export interface DisplayFieldInput {
  key: string;
  label: string;
  valueRef: string;
  format: string;
  unit?: string;
  totalRef?: string;
  displayOrder: number;
  badge?: string;
  badgePresets?: string;
  group?: string;
  groupLabelRef?: string;
}

// Usage Monitor Binding Summary
export const usageMonitorBindingSummarySchema = z.object({
  channelID: z.string(),
  channelName: z.string(),
  usageMonitorChannelID: z.string(),
  strategy: z.enum(['any', 'all']),
  enabled: z.boolean(),
  triggerStatuses: z.array(quotaMonitorBindingTriggerStatusSchema),
  conditions: z.array(quotaMonitorBindingConditionSchema),
  matched: z.boolean(),
  reason: z.string().optional().nullable(),
});
export type UsageMonitorBindingSummary = z.infer<typeof usageMonitorBindingSummarySchema>;
