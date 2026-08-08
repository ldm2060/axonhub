#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const frontendDirectory = path.resolve(scriptDirectory, '..');
const distDirectory = path.resolve(frontendDirectory, '..', 'internal', 'server', 'static', 'dist');
const manifestPath = path.join(distDirectory, '.vite', 'manifest.json');

const budgets = {
  entry: { raw: 640 * 1024, gzip: 200 * 1024 },
  adminModels: { raw: 1_450 * 1024, gzip: 380 * 1024 },
  personalModels: { raw: 1_425 * 1024, gzip: 370 * 1024 },
  modelsProvider: { raw: 60 * 1024, gzip: 15 * 1024 },
  modelsDialogs: { raw: 100 * 1024, gzip: 25 * 1024 },
};

const routeSources = {
  adminModels: 'src/routes/_authenticated/admin/models/index.tsx?tsr-split=component',
  personalModels: 'src/routes/_authenticated/my-models/index.tsx?tsr-split=component',
};

if (!fs.existsSync(manifestPath)) {
  throw new Error(`Vite manifest not found at ${manifestPath}. Run pnpm build first.`);
}

const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
const sizeCache = new Map();
const failures = [];

function assetSizes(file) {
  const cached = sizeCache.get(file);
  if (cached) {
    return cached;
  }

  const contents = fs.readFileSync(path.join(distDirectory, file));
  const sizes = {
    raw: contents.byteLength,
    gzip: zlib.gzipSync(contents, { level: 9 }).byteLength,
  };
  sizeCache.set(file, sizes);
  return sizes;
}

function staticGraph(rootKey) {
  const keys = new Set();

  function visit(key) {
    if (!key || keys.has(key)) {
      return;
    }

    const entry = manifest[key];
    if (!entry) {
      failures.push(`Manifest entry is missing: ${key}`);
      return;
    }

    keys.add(key);
    for (const importedKey of entry.imports ?? []) {
      visit(importedKey);
    }
  }

  visit(rootKey);
  return keys;
}

function graphSizes(keys) {
  let raw = 0;
  let gzip = 0;

  for (const key of keys) {
    const file = manifest[key]?.file;
    if (!file?.endsWith('.js')) {
      continue;
    }

    const sizes = assetSizes(file);
    raw += sizes.raw;
    gzip += sizes.gzip;
  }

  return { raw, gzip };
}

function findManifestKey(predicate, description) {
  const matches = Object.keys(manifest).filter(predicate);
  if (matches.length !== 1) {
    failures.push(`Expected one ${description} manifest entry, found ${matches.length}.`);
    return null;
  }
  return matches[0];
}

function checkBudget(name, sizes) {
  const budget = budgets[name];
  if (sizes.raw > budget.raw) {
    failures.push(`${name} raw size ${formatBytes(sizes.raw)} exceeds ${formatBytes(budget.raw)}.`);
  }
  if (sizes.gzip > budget.gzip) {
    failures.push(`${name} gzip size ${formatBytes(sizes.gzip)} exceeds ${formatBytes(budget.gzip)}.`);
  }
}

function describeEntry(key) {
  const entry = manifest[key];
  return [key, entry?.src, entry?.name, entry?.file].filter(Boolean).join(' ');
}

function checkForbiddenStaticAssets(name, keys) {
  const forbidden = [
    ['Mermaid', /(^|[/_.-])mermaid([/_.-]|$)/i],
    ['Shiki', /(^|[/_.-])shiki([/_.-]|$)|@shikijs(?:\/|$)/i],
    ['WASM', /\.wasm(?:$|\?)/i],
    ['model icon implementations', /@lobehub\/icons\/es\/.*\/components\/Mono\.js/i],
    ['model dialogs', /src\/features\/models\/components\/models-dialogs\.tsx/i],
  ];

  for (const key of keys) {
    const description = describeEntry(key);
    for (const [label, pattern] of forbidden) {
      if (pattern.test(description)) {
        failures.push(`${name} static graph includes demand-loaded ${label}: ${description}`);
      }
    }
  }
}

function formatBytes(bytes) {
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

const entryKey = findManifestKey((key) => manifest[key].isEntry, 'application entry');
const adminModelsKey = findManifestKey((key) => key === routeSources.adminModels, 'admin models route');
const personalModelsKey = findManifestKey((key) => key === routeSources.personalModels, 'personal models route');
const modelsProviderKey = findManifestKey((key) => manifest[key].name === 'models-provider', 'models provider chunk');
const modelsDialogsKey = findManifestKey(
  (key) => key === 'src/features/models/components/models-dialogs.tsx' && manifest[key].isDynamicEntry,
  'lazy models dialogs chunk'
);

const graphKeys = {
  entry: entryKey ? staticGraph(entryKey) : new Set(),
  adminModels: adminModelsKey ? staticGraph(adminModelsKey) : new Set(),
  personalModels: personalModelsKey ? staticGraph(personalModelsKey) : new Set(),
};

const results = {
  entry: graphSizes(graphKeys.entry),
  adminModels: graphSizes(graphKeys.adminModels),
  personalModels: graphSizes(graphKeys.personalModels),
  modelsProvider: modelsProviderKey ? assetSizes(manifest[modelsProviderKey].file) : { raw: 0, gzip: 0 },
  modelsDialogs: modelsDialogsKey ? assetSizes(manifest[modelsDialogsKey].file) : { raw: 0, gzip: 0 },
};

for (const [name, sizes] of Object.entries(results)) {
  checkBudget(name, sizes);
}

for (const [name, keys] of Object.entries(graphKeys)) {
  checkForbiddenStaticAssets(name, keys);
}

const modelIconEntries = Object.entries(manifest).filter(
  ([key, entry]) => /node_modules\/@lobehub\/icons\/es\/.*\/components\/Mono\.js/.test(key) && entry.isDynamicEntry
);
if (modelIconEntries.length < 300) {
  failures.push(`Expected the historical model icon registry to expose at least 300 dynamic entries, found ${modelIconEntries.length}.`);
}

const localeEntries = Object.entries(manifest).filter(
  ([key, entry]) => /^src\/locales\/(?:en|zh-CN)\/.*\.json$/.test(key) && entry.isDynamicEntry
);
if (localeEntries.length === 0) {
  failures.push('Expected locale JSON files to be emitted as dynamic entries.');
}

for (const [name, sizes] of Object.entries(results)) {
  process.stdout.write(`${name.padEnd(16)} raw ${formatBytes(sizes.raw).padStart(12)}  gzip ${formatBytes(sizes.gzip).padStart(12)}\n`);
}
process.stdout.write(`model icons      ${modelIconEntries.length} dynamic entries\n`);
process.stdout.write(`locale resources ${localeEntries.length} dynamic entries\n`);

if (failures.length > 0) {
  process.stderr.write('\nBundle checks failed:\n');
  for (const failure of failures) {
    process.stderr.write(`- ${failure}\n`);
  }
  process.exit(1);
}

process.stdout.write('Bundle checks passed.\n');
