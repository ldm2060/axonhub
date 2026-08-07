import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'preview-chunk-batcher.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { createPreviewChunkBatcher } = await import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);

test('publishes queued chunks in order with one scheduled frame', () => {
  let scheduled;
  let scheduleCount = 0;
  const published = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    (callback) => {
      scheduleCount += 1;
      scheduled = callback;
      return 1;
    },
    () => {}
  );

  batcher.push('a');
  batcher.push('b');
  batcher.push('c');

  assert.equal(scheduleCount, 1);
  scheduled();
  assert.deepEqual(published, [['a', 'b', 'c']]);
});

test('flush publishes immediately and cancels the pending frame', () => {
  const published = [];
  const canceled = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    () => 7,
    (id) => canceled.push(id)
  );

  batcher.push('a');
  batcher.flush();

  assert.deepEqual(canceled, [7]);
  assert.deepEqual(published, [['a']]);
});

test('dispose cancels pending work and drops unpublished chunks', () => {
  let scheduled;
  const published = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    (callback) => {
      scheduled = callback;
      return 9;
    },
    () => {}
  );

  batcher.push('a');
  batcher.dispose();
  scheduled();

  assert.deepEqual(published, []);
});
