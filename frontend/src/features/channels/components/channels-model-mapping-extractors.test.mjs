import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Run the dialog's pure prefix/suffix extractors without loading React or its providers.
const source = readFileSync(new URL('./channels-model-mapping-dialog.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-model-mapping-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const helperNames = ['PREFIX_SEPARATORS', 'extractAllPrefixes', 'extractAllSuffixes'];
const helpers = ast.statements
  .filter((node) => {
    if (ts.isVariableStatement(node)) {
      const name = node.declarationList.declarations[0]?.name?.text;
      return helperNames.includes(name);
    }
    return false;
  })
  .map((node) => `export ${node.getText(ast)}`)
  .join('\n');
const { outputText } = ts.transpileModule(helpers, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { extractAllPrefixes, extractAllSuffixes } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

test('slash-separated prefixes are extracted cumulatively', () => {
  assert.deepEqual(extractAllPrefixes(['deepseek-ai/deepseek-chat', 'a/b/c']), ['a', 'a/b', 'deepseek-ai']);
});

test('colon-separated prefixes are extracted, with or without the separator kept', () => {
  assert.deepEqual(extractAllPrefixes(['cn:deepseek-v4.1-flash', 'cn:deepseek-v4.1-mini']), ['cn']);
  assert.deepEqual(extractAllPrefixes(['qwen3:14b', 'qwen3:32b', 'cn:x']), ['cn', 'qwen3']);
});

test('models without separators yield no prefix suggestions', () => {
  assert.deepEqual(extractAllPrefixes(['gpt-4.1', 'glm-5.1']), []);
  assert.deepEqual(extractAllPrefixes([]), []);
});

test('common suffixes shared by 2+ leaves are detected, including colon tails', () => {
  assert.deepEqual(extractAllSuffixes(['deepseek-v4-pro-free', 'deepseek-v4-lite-free']), ['-free']);
  assert.deepEqual(extractAllSuffixes(['claude-3-5-sonnet:0', 'claude-3-5-haiku:0']), [':0']);
});

test('suffixes unique to a single model are not suggested', () => {
  assert.deepEqual(extractAllSuffixes(['model-a-fast', 'model-b-slow']), []);
  assert.deepEqual(extractAllSuffixes(['only-one-model']), []);
});
