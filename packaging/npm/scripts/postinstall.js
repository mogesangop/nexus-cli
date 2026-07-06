#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');
const { execFileSync } = require('child_process');

const REPO = 'mogesangop/nexus-cli';
const pkg = require('../package.json');
const version = pkg.version.replace(/^v/, '');

const PLATFORM = process.platform;
const ARCH = process.arch;

const SUPPORTED = {
  linux: { x64: true, arm64: true },
  darwin: { x64: true, arm64: true },
  win32: { x64: true, arm64: true },
};

if (!SUPPORTED[PLATFORM] || !SUPPORTED[PLATFORM][ARCH]) {
  console.error(`nexus-cli: unsupported platform/arch ${PLATFORM}/${ARCH}`);
  process.exit(1);
}

const ext = PLATFORM === 'win32' ? 'zip' : 'tar.gz';
const asset = `nexus-cli-${PLATFORM}-${ARCH}.${ext}`;
const base = `https://github.com/${REPO}/releases/download/v${version}`;
const url = `${base}/${asset}`;
const sha256Url = `${url}.sha256`;

const vendorDir = path.join(__dirname, '..', 'vendor');

function fetchBuffer(url, redirects) {
  redirects = redirects || 0;
  if (redirects > 5) return Promise.reject(new Error('too many redirects'));
  return new Promise((resolve, reject) => {
    const req = https.get(url, { headers: { 'User-Agent': 'nexus-cli-npm-postinstall' } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return resolve(fetchBuffer(res.headers.location, redirects + 1));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
      }
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve(Buffer.concat(chunks)));
    });
    req.on('error', reject);
  });
}

async function main() {
  console.log(`nexus-cli: downloading ${asset} (v${version})`);
  const data = await fetchBuffer(url);

  console.log('nexus-cli: verifying sha256');
  const shaFile = (await fetchBuffer(sha256Url)).toString('utf8').trim();
  const expected = shaFile.split(/\s+/)[0];
  const actual = crypto.createHash('sha256').update(data).digest('hex');
  if (actual !== expected) {
    throw new Error(`sha256 mismatch: expected ${expected}, got ${actual}`);
  }

  fs.mkdirSync(vendorDir, { recursive: true });

  if (PLATFORM === 'win32') {
    const zipPath = path.join(vendorDir, 'archive.zip');
    fs.writeFileSync(zipPath, data);
    execFileSync('tar', ['-xf', zipPath, '-C', vendorDir], { stdio: 'inherit' });
    fs.unlinkSync(zipPath);
  } else {
    const tgzPath = path.join(vendorDir, 'archive.tar.gz');
    fs.writeFileSync(tgzPath, data);
    execFileSync('tar', ['-xzf', tgzPath, '-C', vendorDir], { stdio: 'inherit' });
    fs.unlinkSync(tgzPath);
    fs.chmodSync(path.join(vendorDir, 'nexus-cli'), 0o755);
  }

  console.log('nexus-cli: installed');
}

main().catch((err) => {
  console.error('nexus-cli: postinstall failed:', err.message);
  process.exit(1);
});
