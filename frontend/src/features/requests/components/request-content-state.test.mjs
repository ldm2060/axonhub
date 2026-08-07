import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'request-content-state.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { DEFAULT_REQUEST_DETAIL_TAB, nextExpandedExecution } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

test('defaults request details to overview', () => {
  assert.equal(DEFAULT_REQUEST_DETAIL_TAB, 'overview');
});

test('allows only one expanded execution and toggles the current one closed', () => {
  assert.equal(nextExpandedExecution(null, '1'), '1');
  assert.equal(nextExpandedExecution('1', '2'), '2');
  assert.equal(nextExpandedExecution('2', '2'), null);
});
