import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildPersonalChannelWhere,
  filterPersonalChannelRows,
  filterSharedPersonalChannels,
  isPersonalChannelSourceReadOnly,
  type PersonalChannelSource,
} from './personal-channel-source.js';

test('builds owner-first channel filters for personal channel sources', () => {
  const currentUserId = '42';

  assert.deepEqual(buildPersonalChannelWhere('mine', currentUserId, {}), {
    statusIn: ['enabled', 'disabled'],
    ownerID: currentUserId,
  });

  assert.deepEqual(buildPersonalChannelWhere('public', currentUserId, {}), {
    statusIn: ['enabled'],
    ownerIDNEQ: currentUserId,
    visibility: 'published',
  });

  assert.deepEqual(buildPersonalChannelWhere('shared', currentUserId, {}), {
    statusIn: ['enabled'],
    ownerIDNEQ: currentUserId,
    visibility: 'shared',
  });
});

test('preserves explicit status filters for the mine source', () => {
  assert.deepEqual(buildPersonalChannelWhere('mine', '42', { statusIn: ['disabled'] }), {
    statusIn: ['disabled'],
    ownerID: '42',
  });
});

test('applies existing channel filters to every personal channel source', () => {
  const filters = {
    nameContainsFold: 'open',
    typeIn: ['openai'],
    errorMessageNotNil: true,
  };

  assert.deepEqual(buildPersonalChannelWhere('public', '7', filters), {
    nameContainsFold: 'open',
    typeIn: ['openai'],
    errorMessageNotNil: true,
    statusIn: ['enabled'],
    ownerIDNEQ: '7',
    visibility: 'published',
  });
});

test('marks only public and shared personal channel sources as read only', () => {
  const sources: PersonalChannelSource[] = ['public', 'shared', 'mine'];

  assert.deepEqual(sources.map(isPersonalChannelSourceReadOnly), [true, true, false]);
});

test('filters shared channels to enabled channels owned by other users', () => {
  const channels = [
    { id: 'owned', name: 'Owned', status: 'enabled', ownerID: '42', type: 'openai' },
    { id: 'disabled', name: 'Disabled', status: 'disabled', ownerID: '9', type: 'openai' },
    { id: 'enabled', name: 'Enabled', status: 'enabled', ownerID: '9', type: 'openai' },
  ];

  assert.deepEqual(filterSharedPersonalChannels(channels, '42').map((channel) => channel.id), ['enabled']);
});

test('filters personal channel rows with the table filters used by shared channels', () => {
  const channels = [
    { id: 'openai-a', name: 'OpenAI A', type: 'openai', tags: ['fast'], supportedModels: ['gpt-4'] },
    { id: 'openai-b', name: 'OpenAI B', type: 'openai_responses', tags: ['slow'], supportedModels: ['gpt-4.1'] },
    { id: 'anthropic', name: 'Claude', type: 'anthropic', tags: ['fast'], supportedModels: ['claude-3'] },
  ];

  assert.deepEqual(
    filterPersonalChannelRows(channels, {
      nameContainsFold: 'open',
      typeIn: ['openai', 'openai_responses'],
      hasTag: 'fast',
      model: 'gpt-4',
    }).map((channel) => channel.id),
    ['openai-a']
  );
});
