import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'request-navigation-state.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { createNavigationState, flattenNavigationPages, mergeNavigationPage } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

const item = (id) => ({ id });
const pageInfo = (start, end, hasPreviousPage = true, hasNextPage = true) => ({
  startCursor: start,
  endCursor: end,
  hasPreviousPage,
  hasNextPage,
});
const page = (ids, start, end) => ({ items: ids.map(item), pageInfo: pageInfo(start, end) });

test('appends an older page and selects its first item', () => {
  const initial = createNavigationState(page(['5', '4'], 's5', 'e4'), 0);
  const result = mergeNavigationPage(initial, page(['3', '2'], 's3', 'e2'), 'older', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3', '2']);
  assert.equal(result.currentIndex, 2);
});

test('prepends a newer page and selects its last item', () => {
  const initial = createNavigationState(page(['3', '2'], 's3', 'e2'), 0);
  const result = mergeNavigationPage(initial, page(['5', '4'], 's5', 'e4'), 'newer', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3', '2']);
  assert.equal(result.currentIndex, 1);
});

test('deduplicates overlapping request IDs', () => {
  const initial = createNavigationState(page(['5', '4'], 's5', 'e4'), 0);
  const result = mergeNavigationPage(initial, page(['4', '3'], 's4', 'e3'), 'older', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3']);
});

test('retains at most three pages and preserves the retained boundary cursor', () => {
  let state = createNavigationState(page(['8'], 's8', 'e8'), 0);
  state = mergeNavigationPage(state, page(['7'], 's7', 'e7'), 'older', 3);
  state = mergeNavigationPage(state, page(['6'], 's6', 'e6'), 'older', 3);
  state = mergeNavigationPage(state, page(['5'], 's5', 'e5'), 'older', 3);

  assert.equal(state.pages.length, 3);
  assert.deepEqual(flattenNavigationPages(state.pages).map(({ id }) => id), ['7', '6', '5']);
  assert.equal(state.pages[0].pageInfo.startCursor, 's7');
  assert.equal(state.pages[0].pageInfo.hasPreviousPage, true);
  assert.equal(state.pages.at(-1).pageInfo.endCursor, 'e5');
});

test('marks an evicted older range as fetchable after prepending newer pages', () => {
  let state = createNavigationState(page(['5'], 's5', 'e5'), 0);
  state = mergeNavigationPage(state, page(['6'], 's6', 'e6'), 'newer', 3);
  state = mergeNavigationPage(state, page(['7'], 's7', 'e7'), 'newer', 3);
  state = mergeNavigationPage(state, page(['8'], 's8', 'e8'), 'newer', 3);

  assert.equal(state.pages.length, 3);
  assert.deepEqual(flattenNavigationPages(state.pages).map(({ id }) => id), ['8', '7', '6']);
  assert.equal(state.pages.at(-1).pageInfo.hasNextPage, true);
});
