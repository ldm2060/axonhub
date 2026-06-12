const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const source = fs.readFileSync(path.join(__dirname, 'channel-success-rate.tsx'), 'utf8');

test('admin dashboard channel success preview requests a bounded list', () => {
  const limitDeclaration = source.match(/CHANNEL_SUCCESS_RATE_PREVIEW_LIMIT\s*=\s*(\d+)/);
  assert.ok(limitDeclaration, 'expected a named preview limit for the dashboard success-rate card');

  const limit = Number(limitDeclaration[1]);
  assert.ok(limit > 0 && limit <= 10, 'preview limit should stay small enough for the dashboard card');
  assert.match(
    source,
    /useChannelSuccessRates\(\s*CHANNEL_SUCCESS_RATE_PREVIEW_LIMIT\s*,\s*undefined\s*,\s*mode\s*\)/,
    'expected the dashboard card to pass the preview limit into useChannelSuccessRates'
  );
});
