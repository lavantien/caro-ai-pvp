#!/usr/bin/env node

/**
 * Union-merges the cobertura reports from every backend test project and
 * prints the whole-backend line coverage percentage.
 *
 * Each test project instruments every assembly it references, so the same
 * source line appears in several reports; the honest aggregate is the union
 * of hits (any suite exercising a line covers it), matching what a merged
 * go test -coverprofile produced for the Go backend. Generated sources
 * under obj/ (logger source generators) are excluded.
 *
 * Usage: node scripts/backend-coverage.mjs
 */
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const TESTS_DIR = resolve(ROOT, 'backend', 'tests');

const files = new Map(); // basename -> Map(line -> hits)

for (const entry of readdirSync(TESTS_DIR, { withFileTypes: true })) {
  if (!entry.isDirectory()) continue;
  const report = resolve(TESTS_DIR, entry.name, 'coverage', 'coverage.cobertura.xml');
  if (!existsSync(report)) continue;

  const xml = readFileSync(report, 'utf8');
  const classes = /<class name="[^"]+" filename="([^"]+)"[\s\S]*?(?:<\/class>|<lines\/>)/g;
  let cls;
  while ((cls = classes.exec(xml))) {
    if (cls[1].includes('obj/')) continue;
    const base = cls[1].replace(/[\\/]/g, '/').split('/').pop();
    if (!files.has(base)) files.set(base, new Map());
    const hits = files.get(base);
    const lines = /<line number="(\d+)" hits="(\d+)"/g;
    let line;
    while ((line = lines.exec(cls[0]))) {
      hits.set(+line[1], (hits.get(+line[1]) ?? 0) + +line[2]);
    }
  }
}

let valid = 0;
let covered = 0;
for (const lines of files.values()) {
  for (const hits of lines.values()) {
    valid++;
    if (hits > 0) covered++;
  }
}

if (valid === 0) {
  console.error('no cobertura reports found under backend/tests/*/coverage');
  process.exit(1);
}
// String(), not a bare number: FORCE_COLOR environments tint numbers.
console.log(String(Math.round((1000 * covered) / valid) / 10));
