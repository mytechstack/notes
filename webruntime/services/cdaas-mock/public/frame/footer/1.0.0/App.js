// Frame module: footer, v1.0.0.
// Settings: text (text), links (list of { label, url }). Only http(s)
// links are rendered; everything goes through textContent.

const isHttp = (u) => { try { return ['http:', 'https:'].includes(new URL(u).protocol); } catch { return false; } };

export function mount(root, ctx) {
  const { settings = {} } = ctx;
  const text = document.createElement('span');
  text.className = 'wr-footer-text';
  text.textContent = settings.text || '';

  const links = (settings.links || [])
    .filter((l) => l && l.label && isHttp(l.url))
    .map((l) => Object.assign(document.createElement('a'), {
      href: l.url, textContent: l.label, target: '_blank', rel: 'noopener',
    }));

  root.replaceChildren(text, ...links);
}
