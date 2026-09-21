// Web Runtime — the versioned, CDN-delivered browser shell (RFC "Key
// Components"). In this POC it's served directly by the Web Runtime
// Service instead of a real CDN, but its job is the same: read
// __PLATFORM_STATE__ (no extra API call), render nav, and lazy-load
// experiences (RFC goal G1: "build once, deploy everywhere").

const state = window.__PLATFORM_STATE__;

const badgeEl = document.getElementById('slot-badge');
const navEl = document.getElementById('app-nav');
const rootEl = document.getElementById('experience-root');

badgeEl.textContent = `${state.tenant} · ${state.env} · slot=${state.slot}`;

function setActive(link) {
  [...navEl.children].forEach((c) => c.classList.remove('active'));
  link.classList.add('active');
}

async function loadExperience(item) {
  rootEl.innerHTML = '<p class="loading">Loading experience…</p>';
  try {
    const mod = await import(item.experience.remote);
    rootEl.innerHTML = '';
    mod.mount(rootEl, { tenant: state.tenant, item });
  } catch (err) {
    rootEl.innerHTML = `<p class="error">Failed to load experience "${item.id}" from ${item.experience.remote}: ${err}</p>`;
  }
}

function renderNav() {
  const nav = (state.manifest.shell && state.manifest.shell.nav) || [];
  let defaultItem = null;
  nav.forEach((item) => {
    const a = document.createElement('a');
    a.href = '#' + item.path;
    a.textContent = item.label;
    a.addEventListener('click', (e) => {
      e.preventDefault();
      setActive(a);
      loadExperience(item);
    });
    navEl.appendChild(a);
    if (item.default || !defaultItem) defaultItem = { el: a, item };
  });
  if (defaultItem) {
    setActive(defaultItem.el);
    loadExperience(defaultItem.item);
  } else {
    rootEl.innerHTML = '<p class="error">Manifest has no nav items.</p>';
  }
}

renderNav();
