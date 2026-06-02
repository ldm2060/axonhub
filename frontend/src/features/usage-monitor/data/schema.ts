import { z } from 'zod';

export const fieldConfigSchema = z.object({
  key: z.string(),
  label: z.string(),
  path: z.string(),
  type: z.enum(['jsonpath', 'regex']),
  format: z.enum(['percentage', 'fraction', 'number', 'datetime', 'text']),
  totalPath: z.string().optional(),
  unit: z.string().optional(),
  groupIndex: z.array(z.number()).optional(),
  displayOrder: z.number(),
  expression: z.string().optional(),
});

export const variableSchema = z.object({
  key: z.string(),
  path: z.string(),
  type: z.enum(['jsonpath', 'regex']),
  groupIndex: z.array(z.number()).optional(),
});

export const displayFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  valueRef: z.string(),
  format: z.enum(['percentage', 'fraction', 'number', 'datetime', 'text']),
  unit: z.string().optional(),
  totalRef: z.string().optional(),
  displayOrder: z.number(),
  badge: z.string().optional(),
  badgePresets: z.string().optional(),
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
  apiMethod: z.enum(['GET', 'POST']),
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
}
