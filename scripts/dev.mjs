#!/usr/bin/env node

/**
 * Dev bootstrap script — starts backend + frontend and opens the browser.
 *
 * Usage: node scripts/dev.mjs
 */

import { spawn } from 'node:child_process';
import {
	FRONTEND_DIR,
	FRONTEND_URL,
	TIMEOUTS,
	createProcessManager,
	startBackend,
	waitForUrl
} from './lib.mjs';

const mgr = createProcessManager();

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

  await startBackend(mgr);

  // Start frontend
  console.log('Starting frontend...');
  mgr.spawnDaemon('npm', ['run', 'dev'], FRONTEND_DIR, 'frontend');
  await waitForUrl(FRONTEND_URL, TIMEOUTS.frontendReadyMs);
  console.log('Frontend ready.\n');

  // Open browser
  console.log(`Opening ${FRONTEND_URL} ...`);
  openBrowser(FRONTEND_URL);

  console.log('\nPress Ctrl+C to stop.');
}

main().catch((err) => {
  console.error(`Fatal: ${err.message}`);
  mgr.cleanup();
  process.exit(1);
});
