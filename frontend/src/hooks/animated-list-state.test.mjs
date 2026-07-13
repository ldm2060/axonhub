import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'animated-list-state.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { reconcileAnimatedQueue } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

const item = (id, second) => ({ id, createdAt: `2026-07-13T00:00:${String(second).padStart(2, '0')}Z` });

test('caps the pending animation queue to the configured page size', () => {
  const incoming = Array.from({ length: 6 }, (_, index) => item(String(index), index));
  const result = reconcileAnimatedQueue([], incoming, [item('0', 0)], 3);

  assert.deepEqual(result.map(({ id }) => id), ['3', '4', '5']);
});

test('deduplicates displayed and already queued records', () => {
  const result = reconcileAnimatedQueue(
    [item('2', 2)],
    [item('3', 3), item('2', 2), item('1', 1)],
    [item('1', 1)],
    10
  );

  assert.deepEqual(result.map(({ id }) => id), ['2', '3']);
});

test('drops queued records that are absent from the latest server page', () => {
  const result = reconcileAnimatedQueue(
    [item('stale', 2), item('kept', 3)],
    [item('new', 4), item('kept', 3)],
    [item('old', 1)],
    10
  );

  assert.deepEqual(result.map(({ id }) => id), ['kept', 'new']);
});
