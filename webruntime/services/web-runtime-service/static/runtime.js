// Web Runtime — the versioned, CDN-delivered browser shell (RFC "Key
// Components"). It reads __PLATFORM_STATE__ (no extra API call), mounts the
// experience frame (header, nav, footer modules named in the manifest) and
// lazy-loads page experiences.
//
// Frame module contract — each frame module exports mount(el, ctx):
//   ctx = { tenant, env, envLabel, user, version, selectedBy,
//           settings,            // this slot's settings from the manifest
//           theme,               // { primary, headerBg }
//           nav: { items, activeId, navigate(id) } }
//   The nav module may return { setActive(id) } so the runtime can tell it
//   which page is open. If a frame module fails to load, the server-rendered
//   fallback in that slot simply stays.

const state = window.__PLATFORM_STATE__;
const shell = state.manifest.shell || {};
const frame = shell.frame || {};
const pages = shell.nav || [];
const byId = new Map(pages.map((item) => [item.id, item]));

const headerEl = document.getElementById('frame-header');
const navEl = document.getElementById('app-nav');
const footerEl = document.getElementById('frame-footer');
const rootEl = document.getElementById('experience-root');
const badgeEl = document.getElementById('slot-badge');

if (badgeEl) {
  badgeEl.textContent = `${state.tenant} · ${state.env} · v${state.manifestVersion} · ${state.selectedBy} · ${state.user}`;
  badgeEl.dataset.selectedBy = state.selectedBy;
}

let activeId = (navEl.querySelector('a.active') || navEl.querySelector('a') || {}).dataset?.id || (pages[0] && pages[0].id);
let navHandle = null;

// ssrData is only meaningful for the page the server rendered first (see
// ssrdata.go); later navigations let the module fetch its own data.
async function loadExperience(item, ssrData = null) {
  rootEl.replaceChildren(Object.assign(document.createElement('p'), { className: 'loading', textContent: 'Loading…' }));
  try {
    const mod = await import(item.experience.remote);
    rootEl.replaceChildren();
    mod.mount(rootEl, { tenant: state.tenant, item, ssrData });
  } catch (err) {
    rootEl.replaceChildren(Object.assign(document.createElement('p'), {
      className: 'error', textContent: `Could not load "${item.label}" (${item.experience.remote}): ${err}`,
    }));
  }
}

function navigate(id) {
  const item = byId.get(id);
  if (!item || id === activeId) return;
  activeId = id;
  navEl.querySelectorAll('a[data-id]').forEach((a) => a.classList.toggle('active', a.dataset.id === id));
  if (navHandle && typeof navHandle.setActive === 'function') navHandle.setActive(id);
  loadExperience(item);
}

// Fallback nav: the server-rendered links work on their own until (or
// unless) the nav frame module takes over the slot.
navEl.addEventListener('click', (e) => {
  const a = e.target.closest('a[data-id]');
  if (!a) return;
  e.preventDefault();
  navigate(a.dataset.id);
});

function frameContext(slot) {
  return {
    tenant: state.tenant,
    env: state.env,
    envLabel: state.envLabel,
    user: state.user,
    version: state.manifestVersion,
    selectedBy: state.selectedBy,
    settings: (frame[slot] && frame[slot].settings) || {},
    theme: frame.theme || {},
    nav: {
      items: pages.map((p) => ({ id: p.id, label: p.label, icon: p.icon })),
      activeId,
      navigate,
    },
  };
}

async function mountFrame(slot, el) {
  const f = frame[slot];
  if (!f || !f.remote || !el) return null;
  try {
    const mod = await import(f.remote);
    const handle = mod.mount(el, frameContext(slot));
    el.dataset.frameModule = `${f.module}@${f.moduleVersion}`;
    return handle || null;
  } catch (err) {
    console.warn(`[web-runtime] frame ${slot} (${f.remote}) failed to load; keeping the fallback`, err);
    return null;
  }
}

if (!pages.length) {
  rootEl.textContent = 'This manifest has no pages.';
} else {
  // Start the first page and the frame modules in parallel.
  const first = byId.get(activeId) || pages[0];
  loadExperience(first, window.__EXPERIENCE_DATA__ ?? null);
  mountFrame('header', headerEl);
  mountFrame('footer', footerEl);
  mountFrame('nav', navEl).then((handle) => { navHandle = handle; });
}
