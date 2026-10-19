// Records an end-to-end walkthrough of the Web Runtime POC as one video.
// Usage: node demo.js <output.webm>
const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const CP = 'http://localhost:8093';
const OIDC = 'http://localhost:8081';
const WRS = 'http://localhost:8080';
const S3 = 'http://localhost:9000/web-manifests';
const W = 1440, H = 900;

const TENANT = `demo-bank-${Date.now().toString().slice(-4)}`;
const ROLE = `${TENANT}-user`;
const ENVS = ['dev', 'sit', 'perf', 'uat', 'prod'];

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function caption(page, title, body = '') {
  await page.evaluate(([t, b]) => {
    let el = document.getElementById('__demo_caption');
    if (!el) {
      el = document.createElement('div');
      el.id = '__demo_caption';
      el.style.cssText = 'position:fixed;left:50%;bottom:24px;transform:translateX(-50%);z-index:2147483647;' +
        'max-width:1100px;width:calc(100% - 96px);background:rgba(19,32,47,.94);color:#F5F6F2;border-radius:14px;' +
        'pointer-events:none;padding:16px 24px;font:16px/1.45 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;box-shadow:0 8px 30px rgba(0,0,0,.35)';
      document.body.appendChild(el);
    }
    el.innerHTML = `<div style="font-weight:700;font-size:19px;margin-bottom:4px;color:#8FB0FF">${t}</div>` +
      (b ? `<div>${b}</div>` : '');
  }, [title, body]);
}

async function point(page, locator, ms = 700) {
  await locator.evaluate((el) => el.scrollIntoView({ block: 'center', inline: 'nearest' }));
  await sleep(300);
  const box = await locator.boundingBox();
  if (box) {
    await page.evaluate(({ x, y, w, h }) => {
      const r = document.createElement('div');
      r.className = '__demo_ring';
      r.style.cssText = `position:absolute;left:${x + scrollX - 6}px;top:${y + scrollY - 6}px;width:${w + 12}px;height:${h + 12}px;` +
        'border:3px solid #F59E0B;border-radius:10px;z-index:2147483646;pointer-events:none;box-shadow:0 0 0 4px rgba(245,158,11,.25)';
      document.body.appendChild(r);
      setTimeout(() => r.remove(), 1600);
    }, { x: box.x, y: box.y, w: box.width, h: box.height });
  }
  await sleep(ms);
}

async function click(page, locator, ms = 900) {
  await point(page, locator);
  await locator.click();
  await sleep(ms);
}

const consoleUrl = (env) => `${CP}/#${TENANT}/${env}`;
const appUrl = (env, user, slot = 'stable') =>
  `${OIDC}/login?${new URLSearchParams({ tenant: TENANT, role: ROLE, env, user, slot })}`;

async function openConsole(page, env) {
  await page.goto(consoleUrl(env));
  await page.waitForSelector(`#pipeline .cp-stage`);
  await sleep(800);
}

async function showJson(page, url, title, body) {
  await page.goto(url);
  await page.addStyleTag({ content: 'body{font:18px/1.5 ui-monospace,Menlo,monospace;padding:32px;background:#13202F;color:#E6EDF5;white-space:pre-wrap}' });
  await caption(page, title, body);
  await sleep(5000);
}

async function showApp(page, env, user, title, body, slot = 'stable') {
  await page.goto(appUrl(env, user, slot));
  await page.waitForLoadState('networkidle').catch(() => {});
  await sleep(1200);
  await caption(page, title, body);
  await sleep(5500);
}

async function waitStatus(page, sel) {
  await page.waitForFunction((s) => {
    const el = document.querySelector(s);
    return el && /success|error/.test(el.className) && el.textContent.trim();
  }, sel, { timeout: 30000 });
  await sleep(1500);
}

async function promoteTo(page, from, to) {
  const next = page.locator('#promote-next-btn');
  await click(page, next, 400);
  await waitStatus(page, '#release-status');
  await caption(page, `Promote ${from} → ${to}: lands as the candidate`,
    `WCR checks the gate (next environment only, and stable in ${from}) and moves <b>${to}</b>'s candidate pointer. Nothing is copied or rebuilt.`);
  await sleep(2500);
}

async function makeStableHere(page, env) {
  await click(page, page.locator('#promote-btn'), 400);
  await waitStatus(page, '#release-status');
  await caption(page, `${env}: candidate → stable`, `A blue-green switch inside ${env}: one pointer write.`);
  await sleep(2000);
}

async function assign(env, users) {
  const q = new URLSearchParams({ tenant: TENANT, env, users: users.join(',') });
  return (await (await fetch(`${WRS}/debug/assign?${q}`)).json()).assignments;
}

(async () => {
  const out = path.resolve(process.argv[2] || 'demo.webm');
  const videoDir = path.join(__dirname, 'video-tmp');
  fs.rmSync(videoDir, { recursive: true, force: true });

  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: W, height: H },
    recordVideo: { dir: videoDir, size: { width: W, height: H } },
  });
  const page = await context.newPage();
  page.setDefaultTimeout(20000);

  // ---- Intro
  await page.goto(CP);
  await page.waitForSelector('#pipeline .cp-stage');
  await caption(page, 'Web Runtime POC — from tenant creation to production',
    'DE Console (config + module catalog) → WCR builds one immutable manifest → S3 → Web Runtime Service serves it. ' +
    'Releases move a pointer per environment: dev → sit → perf → uat → prod.');
  await sleep(6000);

  // ---- 1. Create a tenant
  await caption(page, '1 · Create a tenant from a template', `Tenant <b>${TENANT}</b>, template <b>banking-standard</b>.`);
  await click(page, page.locator('#new-tenant-btn'));
  await page.locator('#nt-id').pressSequentially(TENANT, { delay: 45 });
  await page.locator('#nt-name').pressSequentially('Demo Bank', { delay: 45 });
  await page.selectOption('#nt-template', 'banking-standard');
  await sleep(1200);
  await click(page, page.locator('#nt-create'), 300);
  await waitStatus(page, '#release-status');
  await page.locator('#pipeline').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, 'v1 is built and live in dev only',
    'The DE Console saved config revision 1 and told WCR. WCR built manifest v1, wrote it to S3 and pointed <b>dev</b> at it. ' +
    'Every other environment says "not deployed" until v1 is promoted there.');
  await point(page, page.locator('#pipeline'), 6000);

  await showApp(page, 'dev', 'alice', 'The app in dev (v1)',
    'Signed in through the OIDC shell. The Web Runtime Service picked v1 from dev\'s pointer and rendered the frame and Home page.');
  await showApp(page, 'prod', 'alice', 'The same tenant in prod: not deployed yet',
    'Nothing reaches prod without passing every environment first.');

  // ---- 2. Promote v1 through every environment
  await openConsole(page, 'dev');
  await page.locator('#pipeline').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '2 · Promote v1 through sit, perf, uat and prod',
    'Each step: promote to the next environment (lands as candidate), then make it stable there.');
  await sleep(3000);
  for (let i = 0; i < ENVS.length - 1; i++) {
    await promoteTo(page, ENVS[i], ENVS[i + 1]);
    await makeStableHere(page, ENVS[i + 1]);
  }
  await caption(page, 'v1 is stable in all five environments', 'The same manifest file everywhere; only the pointers moved.');
  await point(page, page.locator('#pipeline'), 4000);

  await showJson(page, `${S3}/releases/${TENANT}/v1.json`, `S3: releases/${TENANT}/v1.json`,
    'The release record: every environment where v1 has been stable, and the history. The promotion gate reads <b>stableIn</b>.');

  // ---- 3. Change config: experience frame
  await openConsole(page, 'dev');
  const navSlot = page.locator('.cp-slot', { has: page.locator('h3', { hasText: 'Navigation' }) });
  const footerSlot = page.locator('.cp-slot', { has: page.locator('h3', { hasText: 'Footer' }) });
  await navSlot.evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '3 · The tenant owner changes the experience frame',
    'Navigation moves to the top, a new footer text and a new header colour. No code and no JSON: just the DE Console.');
  await sleep(2500);
  const pos = navSlot.locator('label.cp-setting', { hasText: 'Position' }).locator('select');
  await point(page, pos); await pos.selectOption('top'); await sleep(900);
  const ft = footerSlot.locator('label.cp-setting', { hasText: 'Footer text' }).locator('input');
  await point(page, ft); await ft.fill(''); await ft.pressSequentially('© 2026 Demo Bank · Member FDIC', { delay: 35 }); await sleep(600);
  await page.locator('#ft-headerbg').evaluate((el) => { el.value = '#0f766e'; el.dispatchEvent(new Event('input', { bubbles: true })); });
  await point(page, page.locator('#ft-headerbg'), 900);
  const save = page.locator('#save-config');
  await click(page, save, 300);
  await waitStatus(page, '#config-status');
  await caption(page, 'Save & publish → v2 built and live in dev',
    'Config revision 2 → WCR built and validated manifest v2 → dev\'s pointer moved to v2. sit, perf, uat and prod still run v1.');
  await point(page, page.locator('#config-status'), 2500);
  await page.locator('#pipeline').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await point(page, page.locator('#pipeline'), 3500);

  await showApp(page, 'dev', 'alice', 'dev now runs v2', 'Top navigation, new header colour and footer.');
  await showApp(page, 'prod', 'alice', 'prod still runs v1', 'Same tenant, same moment: prod is untouched until v2 is promoted.');

  // ---- 4. Promote v2 up to uat, then into prod as candidate
  await openConsole(page, 'dev');
  await page.locator('#pipeline').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '4 · Promote v2 through the environments', 'Same gates as before.');
  await sleep(2000);
  for (let i = 0; i < ENVS.length - 1; i++) {
    await promoteTo(page, ENVS[i], ENVS[i + 1]);
    if (ENVS[i + 1] !== 'prod') await makeStableHere(page, ENVS[i + 1]);
  }
  await caption(page, 'prod: stable v1, candidate v2', 'Blue-green: v2 is deployed in prod but takes no traffic yet.');
  await point(page, page.locator('.cp-tiles'), 3500);

  await showApp(page, 'prod', 'qa-tester', 'Preview: a tester opens the candidate in prod',
    'With slot=candidate the tester sees v2 in real prod; customers still get v1.', 'candidate');

  // ---- 5. Canary
  await openConsole(page, 'prod');
  const canary = page.locator('#canary-buttons button', { hasText: '25%' });
  await canary.evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '5 · Canary: send 25% of prod users to v2',
    'Each user is hashed to a fixed bucket 0–99; buckets below 25 get v2. The same user always gets the same version.');
  await click(page, canary, 300);
  await waitStatus(page, '#release-status');
  await page.locator('#user-grid').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, 'Who sees what in prod: 100 simulated users',
    'Computed by the Web Runtime Service\'s own selection code. Roughly a quarter are on v2 (canary), the rest on v1.');
  await point(page, page.locator('#user-grid'), 5000);

  const users = Array.from({ length: 60 }, (_, i) => `customer-${i + 1}`);
  const rows = await assign('prod', users);
  const canaryUser = rows.find((r) => r.selectedBy === 'canary').user;
  const stableUser = rows.find((r) => r.selectedBy === 'stable').user;
  await showApp(page, 'prod', canaryUser, `${canaryUser} is in the canary → v2`, 'Their bucket is below 25.');
  await showApp(page, 'prod', stableUser, `${stableUser} stays on stable → v1`, 'Their bucket is 25 or above.');
  await showJson(page, `${S3}/pointers/${TENANT}/prod.json`, `S3: pointers/${TENANT}/prod.json`,
    'The only thing that changed: prod\'s pointer — stable v1, candidate v2, canary 25%.');

  // ---- 6. Promote to stable
  await openConsole(page, 'prod');
  await page.locator('#promote-btn').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '6 · Canary looks healthy: promote v2 to stable in prod', 'Everyone in prod moves to v2.');
  await sleep(1500);
  await click(page, page.locator('#promote-btn'), 300);
  await waitStatus(page, '#release-status');
  await page.locator('#user-grid').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await point(page, page.locator('#user-grid'), 3500);
  await showApp(page, 'prod', stableUser, `prod: ${stableUser} now gets v2`, 'Released to everyone.');

  // ---- 7. Rollback
  await openConsole(page, 'prod');
  const rb = page.locator('#versions-body tr', { hasText: 'v1' }).locator('button', { hasText: 'Make stable in prod' });
  await rb.evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, '7 · Something is wrong: instant rollback in prod',
    'Point prod\'s stable back at v1. Nothing is rebuilt or redeployed; other environments are untouched.');
  await sleep(2000);
  await click(page, rb, 300);
  await waitStatus(page, '#release-status');
  await page.locator('#pipeline').evaluate((el) => el.scrollIntoView({ block: 'center' }));
  await caption(page, 'prod is back on v1 in seconds', 'dev, sit, perf and uat still run v2.');
  await point(page, page.locator('#pipeline'), 4000);
  await showApp(page, 'prod', stableUser, `prod: ${stableUser} is back on v1`, 'Rollback took one pointer write.');

  // ---- Outro
  await openConsole(page, 'prod');
  await caption(page, 'Summary',
    'Config is changed in one place → built once into an immutable manifest → the same file is promoted dev → prod ' +
    'with gates, preview, canary and instant rollback, by moving one pointer per environment.');
  await sleep(7000);

  const video = page.video();
  await context.close();
  await video.saveAs(out);
  await browser.close();
  fs.rmSync(videoDir, { recursive: true, force: true });
  console.log('saved', out, 'tenant', TENANT);
})().catch((e) => { console.error(e); process.exit(1); });
