import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const detailSource = readFileSync(join(import.meta.dirname, 'request-detail-content.tsx'), 'utf8');
const projectPageSource = readFileSync(join(import.meta.dirname, 'request-detail-page.tsx'), 'utf8');
const globalPageSource = readFileSync(join(import.meta.dirname, 'request-detail-global-page.tsx'), 'utf8');
const executionContentSource = readFileSync(join(import.meta.dirname, 'request-execution-content.tsx'), 'utf8');

test('detail routes fetch metadata and default to overview', () => {
  assert.match(projectPageSource, /useRequestMetadata\(/);
  assert.match(globalPageSource, /useRequestMetadata\(/);
  assert.match(projectPageSource, /DEFAULT_REQUEST_DETAIL_TAB/);
  assert.match(globalPageSource, /DEFAULT_REQUEST_DETAIL_TAB/);
});

test('main payload hooks live in tab-owned panels', () => {
  assert.match(detailSource, /function RequestContentPanel[\s\S]*useRequestContent\(/);
  assert.match(detailSource, /function ResponseContentPanel[\s\S]*useRequestContent\(/);
  assert.match(detailSource, /<Tabs value=\{activeTab\}/);
  assert.match(detailSource, /<TabsTrigger value='overview'/);
});

test('response preview is gated by the active response tab', () => {
  assert.match(projectPageSource, /isResponseActive/);
  assert.match(projectPageSource, /!isLivePreviewEnabled \|\| !isResponseActive/);
});

test('overview reuses the metadata usage summary', () => {
  assert.doesNotMatch(detailSource, /useUsageLogs\(/);
  assert.match(detailSource, /request\.usageLogs\?\.edges/);
});

test('overview preserves detailed usage metrics from the metadata summary', () => {
  assert.match(detailSource, /completionReasoningTokens/);
  assert.match(detailSource, /promptWriteCachedTokens/);
  assert.match(detailSource, /cacheHitRate/);
  assert.match(detailSource, /writeCacheRate/);
});

test('execution summaries mount one node-scoped content panel at a time', () => {
  assert.match(detailSource, /expandedExecutionId/);
  assert.match(detailSource, /nextExpandedExecution/);
  assert.match(detailSource, /expandedExecutionId === execution\.id && \(/);
  assert.match(executionContentSource, /useRequestExecutionContent\(/);
});
