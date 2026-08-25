import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

// The Zod schema lives in TypeScript and cannot be imported from node --test,
// so these checks mirror the schema invariants that a catalog sync can break.
// When the schema gains a strictly-typed field, extend the checks here too.

// The test lives at src/features depth so the `node --test src/**/*.test.mjs`
// glob used by CI (POSIX sh without globstar) picks it up.

const srcRoot = join(import.meta.dirname, '..');
const catalog = JSON.parse(readFileSync(join(srcRoot, 'features/models/data/providers.json'), 'utf8'));
const schemaSource = readFileSync(join(srcRoot, 'features/models/data/providers.schema.ts'), 'utf8');

function isPlainObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function checkBoolean(value, path, violations) {
  if (value !== undefined && typeof value !== 'boolean') {
    violations.push(`${path}: expected boolean`);
  }
}

function checkTokenCost(value, path, violations) {
  if (value === undefined) return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected object`);
    return;
  }
  for (const key of ['input', 'output', 'cache_read', 'cache_write']) {
    if (value[key] !== undefined && typeof value[key] !== 'number') {
      violations.push(`${path}.${key}: expected number`);
    }
  }
}

function checkCost(value, path, violations) {
  if (value === undefined) return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected object`);
    return;
  }
  checkTokenCost(value, path, violations);
  checkTokenCost(value.context_over_200k, `${path}.context_over_200k`, violations);
  const tiers = value.tiers;
  if (tiers === undefined) return;
  if (!Array.isArray(tiers)) {
    violations.push(`${path}.tiers: expected array`);
    return;
  }
  tiers.forEach((tier, index) => {
    const tierPath = `${path}.tiers[${index}]`;
    checkTokenCost(tier, tierPath, violations);
    if (!isPlainObject(tier.tier) || typeof tier.tier.type !== 'string') {
      violations.push(`${tierPath}.tier: expected object with string type`);
    }
  });
}

function checkLimit(value, path, violations) {
  if (value === undefined || value === null) return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected object`);
    return;
  }
  for (const key of ['context', 'input', 'output']) {
    const entry = value[key];
    if (entry !== undefined && entry !== null && typeof entry !== 'number') {
      violations.push(`${path}.${key}: expected number or null`);
    }
  }
}

function checkReasoning(value, path, violations) {
  if (value === undefined) return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected object`);
    return;
  }
  for (const key of ['supported', 'default']) {
    checkBoolean(value[key], `${path}.${key}`, violations);
  }
}

function checkReasoningOptions(value, path, violations) {
  if (value === undefined) return;
  if (!Array.isArray(value)) {
    violations.push(`${path}: expected array`);
    return;
  }
  value.forEach((option, index) => {
    if (!isPlainObject(option) || typeof option.type !== 'string') {
      violations.push(`${path}[${index}]: expected object with string type`);
    }
  });
}

function checkModalities(value, path, violations) {
  if (value === undefined) return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected object`);
    return;
  }
  for (const key of ['input', 'output']) {
    const entry = value[key];
    if (entry === undefined || entry === null) continue;
    if (!Array.isArray(entry) || entry.some((item) => typeof item !== 'string')) {
      violations.push(`${path}.${key}: expected string array or null`);
    }
  }
}

// Catalogs use a boolean experimental flag (models.dev style) or an object
// with mode-specific overrides (e.g. Anthropic fast mode).
function checkExperimental(value, path, violations) {
  if (value === undefined || typeof value === 'boolean') return;
  if (!isPlainObject(value)) {
    violations.push(`${path}: expected boolean or object`);
    return;
  }
  const modes = value.modes;
  if (modes === undefined) return;
  if (!isPlainObject(modes)) {
    violations.push(`${path}.modes: expected object`);
    return;
  }
  for (const [mode, override] of Object.entries(modes)) {
    if (!isPlainObject(override)) {
      violations.push(`${path}.modes.${mode}: expected object`);
    }
  }
}

function checkModel(model, path, violations) {
  if (typeof model.id !== 'string') {
    violations.push(`${path}.id: expected string`);
  }
  for (const key of ['attachment', 'tool_call', 'structured_output', 'temperature', 'open_weights', 'vision']) {
    checkBoolean(model[key], `${path}.${key}`, violations);
  }
  checkReasoning(model.reasoning, `${path}.reasoning`, violations);
  checkReasoningOptions(model.reasoning_options, `${path}.reasoning_options`, violations);
  checkModalities(model.modalities, `${path}.modalities`, violations);
  checkCost(model.cost, `${path}.cost`, violations);
  checkLimit(model.limit, `${path}.limit`, violations);
  checkExperimental(model.experimental, `${path}.experimental`, violations);
}

test('bundled providers catalog satisfies the Zod schema invariants', () => {
  assert.ok(isPlainObject(catalog.providers), 'catalog.providers should be an object');
  const violations = [];
  for (const [providerId, provider] of Object.entries(catalog.providers)) {
    if (!isPlainObject(provider)) {
      violations.push(`providers.${providerId}: expected object`);
      continue;
    }
    const models = provider.models;
    if (models === undefined) continue;
    if (!Array.isArray(models)) {
      violations.push(`providers.${providerId}.models: expected array`);
      continue;
    }
    models.forEach((model, index) => checkModel(model, `providers.${providerId}.models[${index}]`, violations));
  }
  assert.deepStrictEqual(violations, [], `catalog schema violations:\n${violations.join('\n')}`);
});

test('providers schema accepts the boolean experimental flag used by catalog syncs', () => {
  assert.match(
    schemaSource,
    /experimental:\s*modelExperimentalFlagSchema\.optional\(\)/,
    'providerModelSchema.experimental must go through the boolean|object union'
  );
  assert.match(
    schemaSource,
    /modelExperimentalFlagSchema\s*=\s*z\.union\(\[\s*z\.boolean\(\),\s*modelExperimentalSchema\s*\]\)/,
    'experimental must accept both the models.dev boolean flag and the mode-override object'
  );
});
