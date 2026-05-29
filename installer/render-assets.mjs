/**
 * render-assets.mjs
 * Renderiza dialog.png y banner.png con Playwright, luego
 * genera el ICO con Python/Pillow y convierte todo a BMP.
 */

import { chromium } from 'playwright';
import { execSync } from 'child_process';
import { existsSync } from 'fs';
import { fileURLToPath } from 'url';
import path from 'path';
import fs from 'fs';

const DIR = 'C:/Users/santi/OneDrive/GDILatam/APP-GDILatam/FirmadorGDI/installer';

function toFileUrl(p) {
  return 'file:///' + p.replace(/\\/g, '/');
}

async function screenshot(page, htmlPath, outPng, width, height) {
  await page.goto(toFileUrl(htmlPath), { waitUntil: 'networkidle' });
  await page.setViewportSize({ width, height });
  // Esperar fuentes
  await page.waitForTimeout(800);
  await page.screenshot({
    path: outPng,
    clip: { x: 0, y: 0, width, height },
    omitBackground: false,
  });
  console.log(`  PNG guardado: ${outPng}`);
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();

  // --- dialog (493x312) ---
  console.log('\n[1/3] Renderizando dialog.png ...');
  const dialogHtml = path.join(DIR, 'dialog-src.html');
  const dialogPng  = path.join(DIR, 'dialog.png');
  await screenshot(page, dialogHtml, dialogPng, 493, 312);

  // --- banner (493x58) ---
  console.log('\n[2/3] Renderizando banner.png ...');
  const bannerHtml = path.join(DIR, 'banner-src.html');
  const bannerPng  = path.join(DIR, 'banner.png');
  await screenshot(page, bannerHtml, bannerPng, 493, 58);

  await browser.close();

  // --- ICO + BMP con Python/Pillow ---
  console.log('\n[3/3] Generando ICO y BMP con Python ...');
  const pyScript = path.join(DIR, 'make-ico-bmp.py');
  execSync(`python "${pyScript}"`, { stdio: 'inherit' });

  console.log('\nListo. Archivos generados:');
  const files = ['dialog.bmp', 'banner.bmp', 'firmadorgdi.ico'];
  for (const f of files) {
    const fp = path.join(DIR, f);
    if (existsSync(fp)) {
      const st = fs.statSync(fp);
      console.log(`  OK  ${fp}  (${(st.size / 1024).toFixed(1)} KB)`);
    } else {
      console.log(`  FALTA  ${fp}`);
    }
  }
}

main().catch(e => { console.error(e); process.exit(1); });
