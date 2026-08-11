import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const parserSource = readFileSync(join(import.meta.dirname, 'request-conversation.ts'), 'utf8');

test('AI SDK format keeps legacy messages with content fields', () => {
  assert.match(parserSource, /fmt\.startsWith\('aisdk'\)[\s\S]*if \(!Array\.isArray\(m\.parts\)\)/);
  assert.match(parserSource, /if \(!Array\.isArray\(m\.parts\)\)[\s\S]*extractOpenAIContent\(m\.content\)/);
  assert.match(parserSource, /if \(!Array\.isArray\(m\.parts\)\)[\s\S]*normalizeToolCalls\(m\.tool_calls\)/);
});
