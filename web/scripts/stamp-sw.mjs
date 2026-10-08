// Runs after `ng build` (Dockerfile): gives the service worker a build id and the list of all JS/CSS files,
// so lazy-loaded pages are cached too and every deploy counts as an update.
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';

const dir = process.argv[2] ?? 'dist/web/browser';
const assets = readdirSync(dir).filter((f) => /\.(js|css)$/.test(f) && f !== 'sw.js').sort();
const file = `${dir}/sw.js`;
const src = readFileSync(file, 'utf8');
if (!src.includes('__BUILD__') || !src.includes('[/*__ASSETS__*/]')) throw new Error('sw.js placeholders missing');
writeFileSync(file, src.replace('__BUILD__', String(Date.now())).replace('[/*__ASSETS__*/]', JSON.stringify(assets)));
console.log(`sw.js stamped with ${assets.length} assets`);
