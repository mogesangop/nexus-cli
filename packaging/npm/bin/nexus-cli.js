#!/usr/bin/env node
'use strict';

const { spawn } = require('child_process');
const path = require('path');

const ext = process.platform === 'win32' ? '.exe' : '';
const bin = path.join(__dirname, '..', 'vendor', `nexus-cli${ext}`);

const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit', windowsHide: false });

child.on('error', (err) => {
  if (err.code === 'ENOENT') {
    console.error('nexus-cli: binary not installed. Run `npm rebuild @mogesang/nexus-cli` or reinstall.');
  } else {
    console.error('nexus-cli:', err);
  }
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  process.exit(code == null ? 1 : code);
});
