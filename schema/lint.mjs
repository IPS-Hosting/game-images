#!/usr/bin/env node
// Lints every <game>/game.yaml in this repository against schema/game-manifest.v2.json
// plus the semantic cross-checks from GAME-MANIFEST-SPEC.md §9.
// Usage: node schema/lint.mjs [--format=github]
import { readdirSync, readFileSync, existsSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import process from 'node:process';
import yaml from 'js-yaml';
import Ajv2020 from 'ajv/dist/2020.js';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SCHEMA = JSON.parse(readFileSync(path.join(REPO, 'schema', 'game-manifest.v2.json'), 'utf8'));

const githubFormat = process.argv.includes('--format=github');
const errors = [];

function error(manifest, message) {
  errors.push({ manifest, message });
  if (githubFormat) console.log(`::error file=${manifest}::${message}`);
  else console.error(`✗ ${manifest}: ${message}`);
}

function collectStrings(node, out = []) {
  if (typeof node === 'string') out.push(node);
  else if (Array.isArray(node)) node.forEach((v) => collectStrings(v, out));
  else if (node && typeof node === 'object') Object.values(node).forEach((v) => collectStrings(v, out));
  return out;
}

const QUOTED = /"[^"]*"|'[^']*'/g;
const IDENT = /[A-Za-z_][A-Za-z0-9_]*/g;

// Context identifiers usable inside templates (spec §4): context + fields + function whitelist.
const TEMPLATE_IDENTIFIERS = new Set([
  // template context
  'Setting', 'SettingBool', 'SettingInt', 'SettingList', 'Port', 'Server', 'Timezone',
  // .Server fields
  'ID', 'IPv4', 'CpuThreads', 'RamMB', 'DiskGB',
  // .Port fields
  'Container', 'Exposed', 'Protocol',
  // function whitelist (spec §4.2)
  'default', 'bool', 'boolInt', 'ternary', 'eq', 'ne', 'lt', 'le', 'gt', 'ge', 'int', 'intf',
  'add', 'sub', 'mul', 'div', 'max', 'min', 'lower', 'upper', 'trim', 'quote', 'replace', 'printf',
  // go template keywords / builtins
  'if', 'else', 'end', 'range', 'with', 'template', 'define', 'block', 'nil',
  'and', 'or', 'not', 'len', 'index', 'slice', 'true', 'false',
]);

const IMAGE_REF = /^[a-z0-9][a-z0-9./:_-]*(:[a-zA-Z0-9._-]+)?$/;

// Discover manifests: <dir>/game.yaml at repo root.
const manifests = readdirSync(REPO)
  .filter((name) => !name.startsWith('.') && name !== 'schema')
  .filter((name) => statSync(path.join(REPO, name)).isDirectory())
  .filter((dir) => existsSync(path.join(REPO, dir, 'game.yaml')))
  .map((dir) => ({ dir, file: path.join(REPO, dir, 'game.yaml') }));

if (manifests.length === 0) {
  console.error('no game.yaml manifests found');
  process.exit(1);
}

const ajv = new Ajv2020({ allErrors: true, strict: false, logger: false }); // strict off: "format": "uri" is documentary only
const validate = ajv.compile(SCHEMA);
const shorts = new Map();

for (const { dir, file } of manifests) {
  const rel = path.relative(REPO, file);
  let manifest;
  try {
    manifest = yaml.load(readFileSync(file, 'utf8'));
  } catch (err) {
    error(rel, `YAML parse error: ${err.message}`);
    continue;
  }
  if (!manifest || typeof manifest !== 'object') {
    error(rel, 'manifest is empty');
    continue;
  }

  if (!validate(manifest)) {
    for (const err of validate.errors) {
      error(rel, `schema: ${err.instancePath || '/'} ${err.message}`);
    }
    continue; // semantic checks below assume a structurally valid manifest
  }

  if (manifest.short !== dir) error(rel, `short "${manifest.short}" must equal directory name "${dir}"`);
  if (shorts.has(manifest.short)) error(rel, `duplicate short "${manifest.short}" (also in ${shorts.get(manifest.short)})`);
  shorts.set(manifest.short, rel);

  const portIds = new Set((manifest.ports ?? []).map((p) => p.identifier));
  for (const id of portIds) {
    const count = (manifest.ports ?? []).filter((p) => p.identifier === id).length;
    if (count > 1) error(rel, `duplicate port identifier "${id}"`);
  }

  // All template strings in the manifest.
  const templates = collectStrings(manifest).filter((s) => s.includes('{{'));

  // Port references: .Port "id" and explicit port identifiers.
  for (const t of templates) {
    for (const m of t.matchAll(/\.Port\s+(?:"([^"]+)"|'([^']+)')/g)) {
      const id = m[1] ?? m[2];
      if (!portIds.has(id)) error(rel, `template references unknown port "${id}": ${t.trim().slice(0, 80)}`);
    }
  }
  const refPort = (id, what) => {
    if (!portIds.has(id)) error(rel, `${what} references unknown port "${id}"`);
  };
  if (manifest.query?.port) refPort(manifest.query.port, 'query.port');
  if (manifest.query?.fallbackPort) refPort(manifest.query.fallbackPort, 'query.fallbackPort');
  if (manifest.stop?.port) refPort(manifest.stop.port, 'stop.port');

  // Setting references.
  const settingKeys = new Set((manifest.settings ?? []).map((s) => s.key));
  for (const t of templates) {
    for (const m of t.matchAll(/\.Setting(Bool|Int|List)?\s+(?:"([^"]+)"|'([^']+)')/g)) {
      const key = m[2] ?? m[3];
      if (!settingKeys.has(key)) {
        error(rel, `template references unknown setting "${key}": ${t.trim().slice(0, 80)}`);
      }
    }
  }

  // Setting integrity.
  for (const setting of manifest.settings ?? []) {
    if (['STRINGENUM', 'NUMBERENUM'].includes(setting.type) && !(setting.values ?? []).length) {
      error(rel, `setting "${setting.key}" (${setting.type}) requires values`);
    }
  }

  // Referenced setting keys must exist.
  const refSetting = (key, what, expectType) => {
    const setting = (manifest.settings ?? []).find((s) => s.key === key);
    if (!setting) {
      error(rel, `${what} references unknown setting "${key}"`);
    } else if (expectType && setting.type !== expectType) {
      error(rel, `${what} setting "${key}" must be ${expectType}, got ${setting.type}`);
    }
  };
  if (manifest.install?.steam?.betaBranchSetting) refSetting(manifest.install.steam.betaBranchSetting, 'install.steam.betaBranchSetting');
  if (manifest.install?.steam?.betaPasswordSetting) refSetting(manifest.install.steam.betaPasswordSetting, 'install.steam.betaPasswordSetting', 'HIDDENSTRING');
  if (manifest.stop?.passwordSetting) refSetting(manifest.stop.passwordSetting, 'stop.passwordSetting', 'HIDDENSTRING');
  if (manifest.query?.passwordSetting) refSetting(manifest.query.passwordSetting, 'query.passwordSetting', 'HIDDENSTRING');
  for (const key of manifest.workshop?.settings ?? []) refSetting(key, 'workshop.settings');
  if (manifest.workshop?.collections) {
    const hasStringSetting = (manifest.workshop.settings ?? []).some((key) =>
      (manifest.settings ?? []).find((s) => s.key === key)?.type === 'STRING');
    if (!hasStringSetting) error(rel, 'workshop.collections requires at least one STRING setting in workshop.settings');
  }

  // fallbackPort is minecraft-auto only (spec §7).
  if (manifest.query?.fallbackPort && manifest.query?.protocol !== 'minecraft') {
    error(rel, 'query.fallbackPort is only allowed with query.protocol minecraft');
  }

  // install.kind none ⇔ flags.installable false (spec §3.2/§9).
  const installable = manifest.flags?.installable ?? true;
  if (manifest.install?.kind === 'none' && installable) error(rel, 'install.kind none requires flags.installable false');
  if (manifest.install?.kind !== 'none' && installable === false) {
    error(rel, 'flags.installable false is only allowed with install.kind none');
  }

  // Image reference: static check with template actions replaced by a placeholder.
  const renderedApprox = manifest.runtime.image.replace(/\{\{[\s\S]*?\}\}/g, 'v0');
  if (!IMAGE_REF.test(renderedApprox)) {
    error(rel, `runtime.image "${renderedApprox}" is not a valid image reference`);
  }

  // Reserved env vars.
  for (const key of Object.keys(manifest.env ?? {})) {
    if (['TZ', 'HOME'].includes(key)) error(rel, `env overrides reserved variable "${key}"`);
  }

  // Requirements must be positive (schema bounds most of this; keep an explicit guard).
  for (const [key, value] of Object.entries(manifest.requirements ?? {})) {
    if (typeof value !== 'number' || value <= 0) error(rel, `requirements.${key} must be a positive number`);
  }
}

if (errors.length) {
  console.error(`\n${errors.length} error(s) in ${manifests.length} manifest(s)`);
  process.exit(1);
}
console.log(`✓ ${manifests.length} manifest(s) valid`);
