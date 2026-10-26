const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {checkNodeVersion} = require('./check-runtime.cjs');

test('accepts patched Node release lines at and above the security baseline', () => {
  for (const version of ['22.23.4', 'v22.23.4', '22.23.5', '22.24.0', '24.22.1', '24.23.0', '26.12.1', '26.13.0']) {
    assert.doesNotThrow(() => checkNodeVersion(version), version);
  }
});

test('rejects vulnerable, unsupported and prerelease Node versions', () => {
  for (const version of ['20.99.0', '22.13.1', '22.23.3', '23.99.0', '24.21.0', '24.22.0', '25.99.0', '26.12.0', '28.0.0', '24.22.1-rc.1', 'invalid']) {
    assert.throws(() => checkNodeVersion(version), /Unsupported build Node.js/, version);
  }
});

test('CLI fails closed on an unsafe runtime and never claims to check Electron', () => {
  let supported = true;
  try { checkNodeVersion(); } catch { supported = false; }
  const result = spawnSync(process.execPath, [path.join(__dirname, 'check-runtime.cjs')], {encoding: 'utf8'});
  assert.equal(result.status, supported ? 0 : 1);
  assert.match(result.stdout + result.stderr, /Electron/);
});

test('both npm projects enforce the same baseline before dependency installation', () => {
  for (const project of ['desktop', 'web']) {
    const root = path.join(__dirname, '..', project);
    const metadata = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
    const lock = JSON.parse(fs.readFileSync(path.join(root, 'package-lock.json'), 'utf8'));
    assert.equal(metadata.engines.node, '^22.23.4 || ^24.22.1 || ^26.12.1');
    assert.equal(lock.packages[''].engines.node, metadata.engines.node);
    assert.match(fs.readFileSync(path.join(root, '.npmrc'), 'utf8'), /^engine-strict=true\s*$/);
  }
});

test('packaging and build entry points retain their early runtime guard', () => {
  const root = path.join(__dirname, '..');
  assert.equal(fs.readFileSync(path.join(root, '.node-version'), 'utf8').trim(), '22.23.4');
  for (const [project, hook] of [['desktop', 'prepack'], ['web', 'prevendor']]) {
    const metadata = JSON.parse(fs.readFileSync(path.join(root, project, 'package.json'), 'utf8'));
    assert.equal(metadata.scripts[hook], 'node ../scripts/check-runtime.cjs');
  }
  for (const name of ['build.ps1', 'build-desktop.ps1']) {
    const script = fs.readFileSync(path.join(__dirname, name), 'utf8');
    const guard = script.indexOf("& node (Join-Path $PSScriptRoot 'check-runtime.cjs')");
    assert.ok(guard >= 0 && guard < script.indexOf('& $go test'));
    assert.match(script.slice(guard, script.indexOf('& $go test')), /if \(\$LASTEXITCODE -ne 0\) \{ throw/);
  }
});
