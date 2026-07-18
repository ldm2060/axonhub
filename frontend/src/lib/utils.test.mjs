import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const source = readFileSync(join(import.meta.dirname, 'utils.ts'), 'utf8');

function loadUtility(name, dependencies = {}) {
  const match = source.match(new RegExp(`export const ${name} = ([\\s\\S]*?\\n};)`));
  assert.ok(match, `${name} should exist`);
  const implementation = match[1].replaceAll(': string', '').replace(/;$/, '');
  return Function(...Object.keys(dependencies), `return (${implementation})`)(...Object.values(dependencies));
}

const extractNumberID = loadUtility('extractNumberID');
const buildGUID = loadUtility('buildGUID', { extractNumberID });

test('buildGUID wraps a numeric ID', () => {
  assert.equal(buildGUID('Request', '197526'), 'gid://axonhub/Request/197526');
});

test('buildGUID normalizes an existing global ID', () => {
  assert.equal(buildGUID('Request', 'gid://axonhub/Request/197526'), 'gid://axonhub/Request/197526');
});
