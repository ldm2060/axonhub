import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'request-query-key.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { buildRequestQueryKey, buildRequestMetadataQueryKey, buildRequestContentQueryKey, buildRequestExecutionContentQueryKey } =
  await import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);

const params = {
  id: 'Request:1',
  permissions: { canViewApiKeys: true },
  projectId: 'Project:1',
  includeAdminFields: false,
};

test('isolates quick-view detail data from the regular request cache', () => {
  const detailKey = buildRequestQueryKey({ ...params, scope: 'detail' });
  const quickViewKey = buildRequestQueryKey({ ...params, scope: 'quick-view' });

  assert.notDeepEqual(quickViewKey, detailKey);
  assert.deepEqual(quickViewKey.slice(0, 2), ['request', 'quick-view']);
});

test('isolates metadata and each main content kind', () => {
  const metadata = buildRequestMetadataQueryKey(params);
  const requestContent = buildRequestContentQueryKey({ ...params, content: 'request' });
  const responseContent = buildRequestContentQueryKey({ ...params, content: 'response' });

  assert.notDeepEqual(metadata, requestContent);
  assert.notDeepEqual(requestContent, responseContent);
  assert.deepEqual(metadata.slice(0, 2), ['request', 'metadata']);
  assert.deepEqual(requestContent.slice(0, 3), ['request', 'content', 'request']);
});

test('isolates execution content by execution id and scope', () => {
  const first = buildRequestExecutionContentQueryKey({ ...params, executionId: 'RequestExecution:1' });
  const second = buildRequestExecutionContentQueryKey({ ...params, executionId: 'RequestExecution:2' });

  assert.notDeepEqual(first, second);
  assert.deepEqual(first.slice(0, 2), ['request-execution', 'content']);
});
