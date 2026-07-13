import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'request-usage-format.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { formatRequestUsageCost } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

test('does not invoke currency formatting without a positive cost and currency code', () => {
  let calls = 0;
  const format = () => {
    calls += 1;
    throw new Error('formatter must not run');
  };

  assert.equal(formatRequestUsageCost(null, undefined, format), '-');
  assert.equal(formatRequestUsageCost(0, 'USD', format), '-');
  assert.equal(formatRequestUsageCost(1, undefined, format), '-');
  assert.equal(calls, 0);
});

test('formats a positive cost when currency settings are ready', () => {
  assert.equal(formatRequestUsageCost(1.25, 'USD', (value, currency) => `${currency} ${value}`), 'USD 1.25');
});
