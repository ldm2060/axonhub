import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

const dialogSource = readFileSync(join(import.meta.dirname, 'channels/components/channels-action-dialog.tsx'), 'utf8');

test('Kimi Code OAuth form hides the endpoint field', () => {
  assert.match(dialogSource, /\{!isKimiCodeType && \(\s*<FormField\s*control=\{form\.control\}\s*name='baseURL'/);
});

test('Kimi Code OAuth form hides the standard API key field', () => {
  assert.match(dialogSource, /isCodexType \|\| isClaudeCodeType \|\| isCopilotType \|\| isKimiCodeType/);
});
