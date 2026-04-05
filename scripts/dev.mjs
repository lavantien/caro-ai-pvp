#!/usr/bin/env node

/**
 * Dev bootstrap script — starts backend + frontend and opens the browser.
 *
 * Usage: node scripts/dev.mjs
 */

import { spawn, spawnSync } from 'node:child_process';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');
const FRONTEND_DIR = resolve(ROOT, 'frontend');

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';
const FRONTEND_URL = process.env.FRONTEND_URL || 'http://localhost:5173';

// --- Process Management ---

/** @type {import('node:child_process').ChildProcess[]} */
const children = [];

function cleanup() {
  for (const child of children) {
    try {
      if (child.pid) {
        if (process.platform === 'win32') {
          spawnSync('taskkill', ['/T', '/F', '/PID', String(child.pid)], {
            stdio: 'ignore',
            shell: false,
          });
        } else {
          process.kill(-child.pid);
        }
      }
    } catch { /* already dead */ }
  }
}

process.on('exit', cleanup);
process.on('SIGINT', () => { cleanup(); process.exit(130); });
process.on('SIGTERM', () => { cleanup(); process.exit(143); });

function runCommand(command, args, cwd, label) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd, shell: true, stdio: ['ignore', 'pipe', 'pipe'] });

    let stderr = '';
    child.stderr?.on('data', (d) => {
      stderr += d.toString();
      if (stderr.length > 10000) stderr = stderr.slice(-5000);
    });

    child.on('error', reject);
    child.on('exit', (code) => {
      if (code === 0) resolve(undefined);
      else reject(new Error(`${label} failed (code ${code}): ${stderr.slice(-500)}`));
    });
  });
}

function spawnDaemon(command, args, cwd, label) {
  const child = spawn(command, args, { cwd, shell: true, stdio: ['ignore', 'pipe', 'pipe'] });

  child.stderr?.on('data', (data) => {
    for (const line of data.toString().split('\n')) {
      if (line.trim()) console.log(`[${label}] ${line}`);
    }
  });

  child.stdout?.on('data', (data) => {
    for (const line of data.toString().split('\n')) {
      if (line.trim()) console.log(`[${label}] ${line}`);
    }
  });

  child.on('error', (err) => console.error(`[${label}] Failed: ${err.message}`));
  child.on('exit', (code) => {
    if (code && code !== 0) console.error(`[${label}] Exited with code ${code}`);
  });

  children.push(child);
  return child;
}

// --- Health Check ---

async function waitForUrl(url, timeoutMs = 30_000, intervalMs = 1000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(url);
      if (resp.ok || resp.status === 404) return;
    } catch { /* not ready */ }
    await new Promise(r => setTimeout(r, intervalMs));
  }
  throw new Error(`Timeout waiting for ${url} (${timeoutMs}ms)`);
}

// --- Open Browser ---

function openBrowser(url) {
  const cmd = process.platform === 'win32' ? 'start'
    : process.platform === 'darwin' ? 'open'
    : 'xdg-open';
  spawn(cmd, [url], { stdio: 'ignore', shell: true, detached: true }).unref();
}

// --- Main ---

async function main() {
  console.log('=== Caro AI PvP - Dev ===\n');

  // Build backend
  console.log('Building backend...');
  await runCommand('dotnet', ['build', 'backend/src/Caro.Api'], ROOT, 'Build');
  console.log('Backend built.\n');

  // Start backend
  console.log('Starting backend...');
  spawnDaemon('dotnet', ['run', '--project', 'backend/src/Caro.Api', '--no-build'], ROOT, 'backend');
  await waitForUrl(`${API_BASE}/`, 60_000);
  console.log('Backend ready.\n');

  // Start frontend
  console.log('Starting frontend...');
  spawnDaemon('npm', ['run', 'dev'], FRONTEND_DIR, 'frontend');
  await waitForUrl(FRONTEND_URL, 30_000);
  console.log('Frontend ready.\n');

  // Open browser
  console.log(`Opening ${FRONTEND_URL} ...`);
  openBrowser(FRONTEND_URL);

  console.log('\nPress Ctrl+C to stop.');
}

main().catch((err) => {
  console.error(`Fatal: ${err.message}`);
  cleanup();
  process.exit(1);
});
