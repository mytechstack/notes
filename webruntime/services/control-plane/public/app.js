// Control-plane UI.
//   1. Configure  → this service's API (/api/configs/{tenant}): config is
//                   authored once per tenant, not per environment.
//   2. Release    → WCR (:8091): each version is built once and promoted
//                   dev → sit → perf → uat → prod; every environment has its
//                   own pointer (stable / candidate / canary).
//   3. Who sees   → Web Runtime Service (:8080) /debug/assign for the
//                   selected environment: the real per-user selection.
// User-editable values are rendered via DOM nodes / textContent only.

const WCR = 'http://localhost:8091';
const WRS = 'http://localhost:8080';
const OIDC = 'http://localhost:8081';
const CANARY_STEPS = [0, 10, 25, 50];

const $ = (id) => document.getElementById(id);

function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'value' || k === 'checked' || k === 'disabled') el[k] = v;
    else el.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    el.append(c instanceof Node ? c : String(c));
  }
  return el;
}

async function api(url, opts) {
  const res = await fetch(url, opts);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(body.error || `HTTP ${res.status}`);
    err.details = body.details || [];
    throw err;
  }
  return body;
}

const post = (url, body) => api(url, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body || {}),
});

function setStatus(el, kind, message, details = []) {
  el.replaceChildren(h('span', {}, message));
  if (details.length) el.append(h('ul', {}, details.map((d) => h('li', {}, d))));
  el.className = 'cp-status' + (kind ? ' cp-status-' + kind : '');
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const clone = (v) => JSON.parse(JSON.stringify(v));
const slugify = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'page';

const state = {
  tenant: null, env: null, modules: {}, templates: {}, config: null, saved: null, revision: 0,
  environments: [], pointers: {}, versions: [],
};

const role = () => (state.config && state.config.roles && state.config.roles[0]) || '';
const nextEnv = (env) => state.environments[state.environments.indexOf(env) + 1] || null;
function loginUrl(opts) {
  return `${OIDC}/login?${new URLSearchParams({ tenant: state.tenant, role: role(), env: state.env, ...opts })}`;
}
function syncHash() {
  history.replaceState(null, '', `#${encodeURIComponent(state.tenant)}/${encodeURIComponent(state.env || '')}`);
}

// ---------------- 1. Configure ----------------

async function loadConfig() {
  const { revision, config } = await api(`/api/configs/${state.tenant}`);
  state.revision = revision;
  state.saved = config;
  state.config = clone(config);
  renderConfig();
}

function renderConfig() {
  $('config-revision').textContent = `config revision ${state.revision}`;
  $('f-roles').value = (state.config.roles || []).join(', ');
  renderPages();
  renderFrame();
}

// ---------------- 2. Experience frame ----------------

const SLOTS = [['header', 'Header'], ['nav', 'Navigation'], ['footer', 'Footer']];

function defaultsFor(moduleId) {
  const out = {};
  ((state.modules[moduleId] || {}).settings || []).forEach((d) => { out[d.key] = clone(d.default); });
  return out;
}

function renderFrame() {
  const frame = state.config.frame || (state.config.frame = { theme: {}, header: {}, nav: {}, footer: {} });
  frame.theme = frame.theme || {};
  $('ft-primary').value = frame.theme.primary || '#1a1a2e';
  $('ft-headerbg').value = frame.theme.headerBg || '#1a1a2e';
  $('frame-slots').replaceChildren(...SLOTS.map(([slot, title]) => renderSlot(slot, title)));
}

function renderSlot(slot, title) {
  const frame = state.config.frame;
  const cur = frame[slot] || (frame[slot] = { module: '', moduleVersion: '', settings: {} });
  const mod = state.modules[cur.module];
  const choices = frameIds(slot);

  const moduleSelect = h('select', {
    'aria-label': `${title} module`,
    onchange: (e) => {
      const next = e.target.value;
      const keep = cur.settings || {};
      cur.module = next;
      cur.moduleVersion = next ? latestOf(next) : '';
      cur.settings = next ? { ...defaultsFor(next), ...Object.fromEntries(Object.entries(keep).filter(([k]) => k in defaultsFor(next))) } : {};
      renderFrame();
    },
  },
  slot === 'footer' ? h('option', { value: '', selected: cur.module ? null : 'selected' }, 'None') : null,
  choices.map((m) => h('option', { value: m, selected: m === cur.module ? 'selected' : null }, m)));

  const versionSelect = cur.module ? h('select', {
    'aria-label': `${title} version`,
    onchange: (e) => { cur.moduleVersion = e.target.value; renderFrame(); },
  }, versionsOf(cur.module).map((v) => h('option', { value: v, selected: v === cur.moduleVersion ? 'selected' : null },
    v + (v === latestOf(cur.module) ? ' (latest)' : '')))) : null;

  const upgrade = cur.module && cur.moduleVersion !== latestOf(cur.module)
    ? h('span', { class: 'cp-upgrade' }, `↑ ${latestOf(cur.module)} available`) : null;

  return h('div', { class: 'cp-slot' },
    h('h3', {}, title),
    h('div', { class: 'cp-slot-pick' }, moduleSelect, versionSelect, upgrade),
    mod ? (mod.settings || []).map((d) => renderSetting(cur, d)) : h('span', { class: 'cp-muted cp-small' }, 'No footer.'));
}

function renderSetting(cur, d) {
  cur.settings = cur.settings || {};
  const val = cur.settings[d.key];
  const set = (v) => { cur.settings[d.key] = v; };
  switch (d.type) {
    case 'bool':
      return h('label', { class: 'cp-check' },
        h('input', { type: 'checkbox', checked: Boolean(val), onchange: (e) => set(e.target.checked) }), d.label);
    case 'select':
      return h('label', { class: 'cp-setting' }, d.label,
        h('select', { onchange: (e) => set(e.target.value) },
          d.options.map((o) => h('option', { value: o, selected: o === val ? 'selected' : null }, o))));
    case 'links': {
      const links = Array.isArray(val) ? val : (cur.settings[d.key] = []);
      return h('div', { class: 'cp-setting cp-links' }, h('span', { class: 'cp-muted cp-small' }, d.label),
        links.map((l, i) => h('div', { class: 'cp-link-row' },
          h('input', { value: l.label || '', placeholder: 'Label', oninput: (e) => { l.label = e.target.value; } }),
          h('input', { type: 'url', value: l.url || '', placeholder: 'https://…', oninput: (e) => { l.url = e.target.value; } }),
          h('button', { title: 'Remove link', onclick: () => { links.splice(i, 1); renderFrame(); } }, '✕'))),
        h('button', { onclick: () => { links.push({ label: '', url: '' }); renderFrame(); } }, '+ Add link'));
    }
    default:
      return h('label', { class: 'cp-setting' }, d.label,
        h('input', {
          type: d.type === 'url' ? 'url' : 'text', value: val ?? '',
          placeholder: d.type === 'url' ? 'https://…' : '', oninput: (e) => set(e.target.value),
        }));
  }
}

const semverDesc = (a, b) => b.split('.').map(Number).reduce((r, n, i) => r || n - (a.split('.').map(Number)[i] || 0), 0);
const versionsOf = (m) => Object.keys((state.modules[m] || {}).versions || {}).sort(semverDesc);
const latestOf = (m) => (state.modules[m] || {}).latest;

const experienceIds = () => Object.keys(state.modules).filter((m) => state.modules[m].kind !== 'frame').sort();
const frameIds = (slot) => Object.keys(state.modules).filter((m) => state.modules[m].kind === 'frame' && state.modules[m].slot === slot).sort();

function renderPages() {
  const pages = state.config.pages;
  const moduleIds = experienceIds();
  $('pages-body').replaceChildren(...pages.map((p, i) =>
    h('tr', {},
      h('td', {}, h('input', {
        type: 'radio', name: 'default-page', checked: p.default,
        onchange: () => pages.forEach((q, j) => { q.default = j === i; }),
      })),
      h('td', {}, h('input', {
        value: p.label,
        oninput: (e) => {
          p.label = e.target.value;
          if (p._new) p.id = uniqueId(slugify(p.label), p);
        },
      })),
      h('td', {}, h('select', {
        onchange: (e) => { p.module = e.target.value; p.moduleVersion = latestOf(p.module); renderPages(); },
      }, moduleIds.map((m) => h('option', { value: m, selected: m === p.module ? 'selected' : null }, m)))),
      h('td', {},
        h('select', { onchange: (e) => { p.moduleVersion = e.target.value; renderPages(); } },
          versionsOf(p.module).map((v) => h('option', { value: v, selected: v === p.moduleVersion ? 'selected' : null },
            v + (v === latestOf(p.module) ? ' (latest)' : '')))),
        p.moduleVersion !== latestOf(p.module)
          ? h('div', { class: 'cp-upgrade' }, `↑ ${latestOf(p.module)} available`) : null),
      h('td', { class: 'cp-actions' },
        h('button', { title: 'Move up', disabled: i === 0, onclick: () => move(i, -1) }, '↑'),
        h('button', { title: 'Move down', disabled: i === pages.length - 1, onclick: () => move(i, 1) }, '↓'),
        h('button', { title: 'Remove', disabled: pages.length === 1, onclick: () => remove(i) }, '✕'))
    )));
}

function uniqueId(id, self) {
  const taken = new Set(state.config.pages.filter((q) => q !== self).map((q) => q.id));
  let candidate = id;
  for (let n = 2; taken.has(candidate); n++) candidate = `${id}-${n}`;
  return candidate;
}

function move(i, d) {
  const [p] = state.config.pages.splice(i, 1);
  state.config.pages.splice(i + d, 0, p);
  renderPages();
}

function remove(i) {
  const pages = state.config.pages;
  const wasDefault = pages[i].default;
  pages.splice(i, 1);
  if (wasDefault) pages[0].default = true;
  renderPages();
}

function addPage() {
  const module = experienceIds()[0];
  const page = { label: 'New page', icon: '', default: false, module, moduleVersion: latestOf(module), _new: true };
  page.id = uniqueId('new-page', page);
  state.config.pages.push(page);
  renderPages();
}

async function saveAndPublish() {
  const status = $('config-status');
  const cfg = clone(state.config);
  cfg.roles = $('f-roles').value.split(',').map((r) => r.trim()).filter(Boolean);
  cfg.pages.forEach((p) => delete p._new);
  let saved;
  try {
    saved = await api(`/api/configs/${state.tenant}`, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg),
    });
  } catch (err) {
    setStatus(status, 'error', `Not saved: ${err.message}`, err.details);
    return;
  }
  state.config = cfg;
  state.saved = clone(cfg);
  state.revision = saved.revision;
  renderConfig();
  setStatus(status, null, `Saved config revision ${saved.revision}. WCR is building the manifest…`);

  const e = await waitForBuild(saved.eventId);
  if (!e) {
    setStatus(status, 'error', `Saved revision ${saved.revision}, but WCR hasn't built it yet (its event is queued and will retry).`);
  } else if (e.status === 'error') {
    setStatus(status, 'error', `WCR rejected revision ${saved.revision}: ${e.error}`);
  } else {
    state.env = state.environments[0];
    setStatus(status, 'success',
      `v${e.version} built from revision ${saved.revision} and live in ${state.env}. Promote it onward from the pipeline below.`);
  }
  await refreshRelease();
}

// Wait for WCR to process a config.changed event (build → S3 → dev).
async function waitForBuild(eventId) {
  for (let i = 0; i < 30; i++) {
    await sleep(500);
    const { events } = await api(`${WCR}/events`).catch(() => ({ events: [] }));
    const e = events.find((x) => x.event.id === eventId);
    if (e && e.status !== 'processing') return e;
  }
  return null;
}

// ---------------- New tenant ----------------

function openNewTenant() {
  $('new-tenant-card').hidden = false;
  setStatus($('nt-status'), null, '');
  $('nt-id').focus();
}

function renderTemplateOptions() {
  const names = Object.keys(state.templates).sort();
  $('nt-template').replaceChildren(...names.map((n) => h('option', { value: n }, n)));
  showTemplateDesc();
}

function showTemplateDesc() {
  const t = state.templates[$('nt-template').value];
  $('nt-template-desc').textContent = t
    ? `${t.description} — pages: ${t.pages.map((p) => p.label).join(', ')} (each pinned to the module's latest version).`
    : '';
}

async function createTenant() {
  const status = $('nt-status');
  const tenantId = $('nt-id').value.trim();
  let created;
  try {
    created = await post('/api/tenants', {
      tenantId,
      displayName: $('nt-name').value.trim(),
      template: $('nt-template').value,
      role: $('nt-role').value.trim(),
    });
  } catch (err) {
    setStatus(status, 'error', err.message, err.details);
    return;
  }
  setStatus(status, null, `Created ${created.tenantId} (config revision ${created.revision}). WCR is building v1…`);
  const e = await waitForBuild(created.eventId);
  if (!e || e.status === 'error') {
    setStatus(status, 'error', e ? `WCR rejected it: ${e.error}` : 'WCR has not built it yet; it will retry.');
    return;
  }
  const { tenants } = await api('/api/tenants');
  $('tenant-select').replaceChildren(...tenants.map((t) => h('option', { value: t }, t)));
  $('tenant-select').value = created.tenantId;
  state.env = state.environments[0] || 'dev';
  $('new-tenant-card').hidden = true;
  ['nt-id', 'nt-name', 'nt-role'].forEach((id) => { $(id).value = ''; });
  await onTenantChange();
  setStatus($('release-status'), 'success',
    `${created.tenantId} v${e.version} is live in ${state.env} only (users need role "${created.role}"). Promote it to reach other environments.`);
}

// ---------------- 2. Release ----------------

async function refreshRelease() {
  const data = await api(`${WCR}/manifests/${state.tenant}/versions`);
  state.environments = data.environments;
  state.pointers = data.pointers;
  state.versions = data.versions;
  if (!state.environments.includes(state.env)) state.env = state.environments[0];
  syncHash();
  renderPipeline();
  renderEnvPanel();
  renderVersions();
  await refreshGrid();
}

function renderPipeline() {
  const cells = [];
  state.environments.forEach((env, i) => {
    const p = state.pointers[env];
    if (i > 0) cells.push(h('span', { class: 'cp-arrow' }, '→'));
    cells.push(h('button', {
      class: 'cp-stage' + (env === state.env ? ' cp-stage-on' : '') + (p.stable ? '' : ' cp-stage-empty'),
      onclick: () => { state.env = env; syncHash(); renderPipeline(); renderEnvPanel(); renderVersions(); refreshGrid(); },
    },
    h('span', { class: 'cp-stage-name' }, env),
    h('strong', {}, p.stable ? `v${p.stable}` : 'not deployed'),
    p.candidate && p.candidate !== p.stable ? h('span', { class: 'cp-stage-cand' }, `candidate v${p.candidate}`) : null,
    p.canary.percent ? h('span', { class: 'cp-stage-canary' }, `canary ${p.canary.percent}% → v${p.canary.version}`) : null));
  });
  $('pipeline').replaceChildren(...cells);
}

function renderEnvPanel() {
  const env = state.env;
  const p = state.pointers[env];
  $('env-title').textContent = `Environment: ${env}`;
  $('t-stable').textContent = p.stable ? `v${p.stable}` : '—';
  $('t-candidate').textContent = !p.candidate ? '—' : p.candidate === p.stable ? `v${p.candidate} (= stable)` : `v${p.candidate}`;
  $('t-canary').textContent = p.canary.percent ? `${p.canary.percent}% → v${p.canary.version}` : 'off';

  const canCanary = p.candidate && p.candidate !== p.stable;
  $('canary-buttons').replaceChildren(...CANARY_STEPS.map((pct) =>
    h('button', {
      class: (p.canary.percent || 0) === pct ? 'cp-on' : null,
      disabled: pct > 0 && !canCanary,
      onclick: () => setCanary(pct),
    }, pct === 0 ? 'Off' : `${pct}%`)));
  $('promote-btn').disabled = !canCanary;
  $('promote-btn').textContent = `Promote candidate → stable in ${env}`;
  $('open-stable').href = loginUrl({ slot: 'stable' });
  $('open-candidate').href = loginUrl({ slot: 'candidate' });

  // Promote this environment's stable version to the next environment.
  const next = nextEnv(env);
  const btn = $('promote-next-btn');
  let reason = '';
  if (!next) reason = `${env} is the last environment`;
  else if (!p.stable) reason = `nothing stable in ${env}`;
  else if (state.pointers[next].candidate === p.stable || state.pointers[next].stable === p.stable) {
    reason = `v${p.stable} is already in ${next}`;
  }
  btn.textContent = next ? `Promote v${p.stable} → ${next}` : 'Promote';
  btn.disabled = Boolean(reason);
  btn.onclick = () => promoteToNext(p.stable, env, next);
  $('promote-next-reason').textContent = reason;
}

function renderVersions() {
  const env = state.env;
  const p = state.pointers[env];
  $('versions-body').replaceChildren(...state.versions.slice().reverse().map((v) =>
    h('tr', {},
      h('td', {}, h('strong', {}, `v${v.version}`)),
      h('td', {}, v.sourceRevision ? `revision ${v.sourceRevision}` : '—'),
      h('td', {}, state.environments.map((e) =>
        h('span', { class: 'cp-envchip' + (v.stableIn.includes(e) ? ' cp-envchip-on' : '') }, e))),
      h('td', {},
        p.stable === v.version ? h('span', { class: 'cp-badge cp-b-stable' }, 'stable') : null,
        p.candidate === v.version && p.candidate !== p.stable ? h('span', { class: 'cp-badge cp-b-candidate' }, 'candidate') : null,
        p.canary.percent && p.canary.version === v.version ? h('span', { class: 'cp-badge cp-b-canary' }, `canary ${p.canary.percent}%`) : null),
      h('td', { class: 'cp-actions' },
        h('button', { onclick: () => viewVersion(v.version) }, 'View'),
        h('button', {
          disabled: p.stable === v.version,
          title: `Point ${env}'s stable at this version (instant rollback / roll forward)`,
          onclick: () => makeStable(v.version),
        }, `Make stable in ${env}`)))));
  $('pointer-key').textContent = `pointers/${state.tenant}/${env}.json`;
  $('pointer-view').textContent = JSON.stringify(p, null, 2);
}

async function releaseAction(fn, okMessage) {
  const status = $('release-status');
  try {
    await fn();
    setStatus(status, 'success', okMessage);
    await refreshRelease();
  } catch (err) {
    setStatus(status, 'error', err.message);
  }
}

const envUrl = (action) => `${WCR}/manifests/${state.tenant}/${state.env}/${action}`;

const setCanary = (percent) => releaseAction(
  () => post(envUrl('canary'), { percent }),
  percent ? `${state.env}: ${percent}% of users now get the candidate.` : `${state.env}: canary stopped — everyone is on stable.`);

const promote = () => releaseAction(
  () => post(envUrl('promote')),
  `${state.env}: candidate promoted — everyone in ${state.env} is on it.`);

const makeStable = (version) => releaseAction(
  () => post(envUrl('rollback'), { version, slot: 'stable' }),
  `${state.env}: stable now points at v${version}. Only the pointer changed; other environments are untouched.`);

async function promoteToNext(version, from, to) {
  await releaseAction(
    () => post(`${WCR}/manifests/${state.tenant}/promote`, { version, from, to }),
    `v${version} is now the candidate in ${to}. Preview it, run a canary, or promote it to stable there.`);
  state.env = to;
  syncHash();
  renderPipeline(); renderEnvPanel(); renderVersions(); refreshGrid();
}

async function viewVersion(n) {
  const view = $('manifest-view');
  view.hidden = false;
  try {
    const m = await api(`${WCR}/manifests/${state.tenant}/versions/${n}`);
    view.textContent = `// manifests/${state.tenant}/v${n}.json — immutable, environment-agnostic, generated by WCR\n`
      + JSON.stringify(m, null, 2);
  } catch (err) {
    view.textContent = `Could not load v${n}: ${err.message}`;
  }
}

// ---------------- 3. Who sees what ----------------

const USERS = Array.from({ length: 100 }, (_, i) => `user-${i + 1}`);

async function assign(users) {
  const q = new URLSearchParams({ tenant: state.tenant, env: state.env, users: users.join(',') });
  return (await api(`${WRS}/debug/assign?${q}`)).assignments;
}

async function refreshGrid() {
  $('grid-env').textContent = `in ${state.env}`;
  if (!(state.pointers[state.env] || {}).stable) {
    $('grid-legend').replaceChildren();
    $('user-grid').replaceChildren(h('div', { class: 'cp-empty' },
      `Nothing is stable in ${state.env} yet, so no user is served here. Promote a version into ${state.env} first.`));
    return;
  }
  try {
    const rows = await assign(USERS);
    const counts = {};
    rows.forEach((r) => { const k = `v${r.version} · ${r.selectedBy}`; counts[k] = (counts[k] || 0) + 1; });
    $('grid-legend').replaceChildren(...Object.entries(counts).map(([k, n]) =>
      h('span', { class: 'cp-legend-item ' + (k.endsWith('canary') ? 'cp-c-canary' : 'cp-c-stable') }, `${k}: ${n}`)));
    $('user-grid').replaceChildren(...rows.map((r) =>
      h('a', {
        class: 'cp-cell ' + (r.selectedBy === 'canary' ? 'cp-c-canary' : 'cp-c-stable'),
        href: loginUrl({ user: r.user }), target: '_blank', rel: 'noopener',
        title: `${r.user} · bucket ${r.bucket} · v${r.version} (${r.selectedBy})`,
      }, String(r.version))));
  } catch (err) {
    $('user-grid').replaceChildren(h('span', { class: 'cp-err' }, `Web Runtime Service unavailable: ${err.message}`));
  }
}

async function lookup() {
  const user = $('lookup-user').value.trim();
  if (!user) return;
  try {
    const [r] = await assign([user]);
    $('lookup-result').replaceChildren(
      `${r.user} in ${state.env} → bucket ${r.bucket} → `, h('strong', {}, `v${r.version}`), ` (${r.selectedBy}) `,
      h('a', { href: loginUrl({ user }), target: '_blank', rel: 'noopener' }, 'open as this user ↗'));
  } catch (err) {
    $('lookup-result').textContent = err.message;
  }
}

// ---------------- wiring ----------------

async function onTenantChange() {
  state.tenant = $('tenant-select').value;
  setStatus($('config-status'), null, '');
  setStatus($('release-status'), null, '');
  $('manifest-view').hidden = true;
  try {
    await loadConfig();
    await refreshRelease();
  } catch (err) {
    setStatus($('release-status'), 'error', err.message);
  }
}

async function init() {
  try {
    state.modules = (await api('/api/modules')).modules;
    state.templates = (await api('/api/templates')).templates;
    renderTemplateOptions();
    const { tenants } = await api('/api/tenants');
    $('tenant-select').replaceChildren(...tenants.map((t) => h('option', { value: t }, t)));
    const [hashTenant, hashEnv] = decodeURIComponent(location.hash.slice(1)).split('/');
    if (tenants.includes(hashTenant)) $('tenant-select').value = hashTenant;
    state.env = hashEnv || null;
    if (tenants.length) await onTenantChange();
  } catch (err) {
    setStatus($('config-status'), 'error', `Control plane unavailable: ${err.message}`);
  }
}

$('tenant-select').addEventListener('change', onTenantChange);
$('ft-primary').addEventListener('input', (e) => { state.config.frame.theme.primary = e.target.value; });
$('ft-headerbg').addEventListener('input', (e) => { state.config.frame.theme.headerBg = e.target.value; });
$('new-tenant-btn').addEventListener('click', openNewTenant);
$('nt-cancel').addEventListener('click', () => { $('new-tenant-card').hidden = true; });
$('nt-create').addEventListener('click', createTenant);
$('nt-template').addEventListener('change', showTemplateDesc);
$('nt-id').addEventListener('input', () => { $('nt-role').placeholder = `${$('nt-id').value.trim() || 'tenant'}-user`; });
$('add-page').addEventListener('click', addPage);
$('save-config').addEventListener('click', saveAndPublish);
$('reset-config').addEventListener('click', () => { state.config = clone(state.saved); renderConfig(); });
$('promote-btn').addEventListener('click', promote);
$('lookup-btn').addEventListener('click', lookup);
$('lookup-user').addEventListener('keydown', (e) => { if (e.key === 'Enter') lookup(); });

init();
