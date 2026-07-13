import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'requests.ts'), 'utf8');
const graphqlSource = readFileSync(join(import.meta.dirname, '../../../gql/graphql.ts'), 'utf8');

function extractFunction(name) {
  const marker = `function ${name}`;
  const start = source.indexOf(marker);
  assert.notEqual(start, -1, `${name} should exist`);
  const signatureEnd = source.indexOf(') {', start);
  assert.notEqual(signatureEnd, -1, `${name} should have a body`);
  const bodyStart = signatureEnd + 2;

  let depth = 0;
  let quote = null;
  let escaped = false;
  for (let index = bodyStart; index < source.length; index += 1) {
    const char = source[index];
    if (quote) {
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === quote) quote = null;
      continue;
    }
    if (char === "'" || char === '"' || char === '`') {
      quote = char;
      continue;
    }
    if (char === '{') depth += 1;
    if (char === '}') {
      depth -= 1;
      if (depth === 0) return source.slice(start, index + 1);
    }
  }
  throw new Error(`${name} body is incomplete`);
}

test('metadata query omits request and response payloads', () => {
  const query = extractFunction('buildRequestMetadataQuery');
  for (const field of ['requestHeaders', 'requestBody', 'responseBody', 'responseChunks']) {
    assert.doesNotMatch(query, new RegExp(`\\b${field}\\b`));
  }
});

test('request and response content queries select only their own payloads', () => {
  const requestQuery = extractFunction('buildRequestContentQuery');
  assert.match(requestQuery, /requestHeaders/);
  assert.match(requestQuery, /requestBody/);
  assert.doesNotMatch(requestQuery, /responseBody|responseChunks/);

  const responseQuery = extractFunction('buildResponseContentQuery');
  assert.match(responseQuery, /responseBody/);
  assert.match(responseQuery, /responseChunks/);
  assert.doesNotMatch(responseQuery, /requestHeaders|requestBody/);
});

test('execution summary query omits payloads and content query is node-scoped', () => {
  const summary = extractFunction('buildRequestExecutionSummariesQuery');
  assert.doesNotMatch(summary, /requestHeaders|requestBody|responseBody|responseChunks/);

  const content = extractFunction('buildRequestExecutionContentQuery');
  assert.match(content, /query GetRequestExecutionContent\(\$id: ID!\)/);
  assert.match(content, /node\(id: \$id\)/);
});

test('content hooks use immediate garbage collection', () => {
  assert.match(source, /useRequestContent[\s\S]*?gcTime:\s*0/);
  assert.match(source, /useRequestExecutionContent[\s\S]*?gcTime:\s*0/);
});

test('graphql client forwards an optional abort signal', () => {
  assert.match(graphqlSource, /signal\?: AbortSignal/);
  assert.match(graphqlSource, /fetch\(GRAPHQL_ENDPOINT,[\s\S]*?signal,/);
});
