import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'request-query-key.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { buildRequestQueryKey } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

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
