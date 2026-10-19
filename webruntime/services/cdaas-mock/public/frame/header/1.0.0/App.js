// Frame module: header, v1.0.0.
// Settings: title (text), logoUrl (url), showUser (bool).
// Built with DOM nodes + textContent only; a logo is shown only for http(s).

const isHttp = (u) => { try { return ['http:', 'https:'].includes(new URL(u).protocol); } catch { return false; } };

function el(tag, props = {}, ...children) {
  const n = Object.assign(document.createElement(tag), props);
  n.append(...children.filter(Boolean));
  return n;
}

export function mount(root, ctx) {
  const { settings = {} } = ctx;
  const brand = el('span', { className: 'wr-brand' },
    isHttp(settings.logoUrl) ? el('img', { className: 'wr-logo-img', src: settings.logoUrl, alt: '' }) : null,
    el('span', { className: 'wr-logo', textContent: settings.title || ctx.tenant }));

  const badge = settings.showUser
    ? el('span', {
        className: 'wr-badge',
        textContent: `${ctx.user} · ${ctx.env} · v${ctx.version}${ctx.selectedBy !== 'stable' ? ' · ' + ctx.selectedBy : ''}`,
      })
    : null;
  if (badge) badge.dataset.selectedBy = ctx.selectedBy;

  root.replaceChildren(brand, badge);
}
