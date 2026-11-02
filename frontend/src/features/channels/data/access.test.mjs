import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const source = readFileSync(join(import.meta.dirname, 'access.ts'), 'utf8');

const extractNumberIDAsNumber = (id) => Number(id.slice(id.lastIndexOf('/') + 1));

function loadCanUseChannel() {
  const match = source.match(/export function canUseChannel\(([\s\S]*?)\): boolean \{([\s\S]*?)\n\}/);
  assert.ok(match, 'canUseChannel should exist');

  const params = match[1].replaceAll(': ChannelAccessFields', '').replaceAll(': AccessUser', '');
  const body = match[2];

  return Function('extractNumberIDAsNumber', `return function canUseChannel(${params}) {${body}\n}`)(extractNumberIDAsNumber);
}

const canUseChannel = loadCanUseChannel();
const me = { id: 'gid://axonhub/User/2' };

test('no acting user has no access', () => {
  assert.equal(canUseChannel({ ownerID: 'gid://axonhub/User/1', visibility: 'published', sharedWith: [] }, null), false);
});

test('the system owner and admins reach every channel', () => {
  const channel = { ownerID: 'gid://axonhub/User/1', visibility: 'private', sharedWith: [] };
  assert.equal(canUseChannel(channel, { ...me, isOwner: true }), true);
  assert.equal(canUseChannel(channel, { ...me, isAdmin: true }), true);
});

test('the channel owner reaches their private channel', () => {
  assert.equal(canUseChannel({ ownerID: me.id, visibility: 'private', sharedWith: [] }, me), true);
});

test('another user cannot reach a private channel', () => {
  assert.equal(canUseChannel({ ownerID: 'gid://axonhub/User/1', visibility: 'private', sharedWith: [] }, me), false);
});

test('published channels are open to everyone', () => {
  assert.equal(canUseChannel({ ownerID: 'gid://axonhub/User/1', visibility: 'published', sharedWith: [] }, me), true);
});

test('shared channels are only open to the listed users', () => {
  assert.equal(canUseChannel({ ownerID: 'gid://axonhub/User/1', visibility: 'shared', sharedWith: [2] }, me), true);
  assert.equal(canUseChannel({ ownerID: 'gid://axonhub/User/1', visibility: 'shared', sharedWith: [3] }, me), false);
});

test('unowned channels stay open', () => {
  assert.equal(canUseChannel({ ownerID: null, visibility: 'private', sharedWith: [] }, me), true);
});
