// Frame module: navigation, v1.0.0.
// Settings: position ("left" | "top" — the server already applied the
// layout class, so nothing jumps), showIcons (bool).
// Renders ctx.nav.items, calls ctx.nav.navigate(id) on click, and returns
// { setActive(id) } so the runtime can tell it which page is open.

const ICONS = { home: '⌂', bank: '▤', list: '≡', arrows: '⇄', gear: '⚙', chart: '▦', gift: '✦' };

export function mount(root, ctx) {
  const { settings = {}, nav } = ctx;
  const links = nav.items.map((item) => {
    const a = document.createElement('a');
    a.href = `#/${item.id}`;
    a.dataset.id = item.id;
    if (settings.showIcons) {
      const icon = document.createElement('span');
      icon.className = 'wr-icon';
      icon.setAttribute('aria-hidden', 'true');
      icon.textContent = ICONS[item.icon] || '•';
      a.append(icon);
    }
    a.append(document.createTextNode(item.label));
    a.addEventListener('click', (e) => {
      e.preventDefault();
      nav.navigate(item.id);
    });
    return a;
  });

  const setActive = (id) => links.forEach((a) => {
    const on = a.dataset.id === id;
    a.classList.toggle('active', on);
    if (on) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
  });

  root.setAttribute('aria-label', 'Main');
  root.replaceChildren(...links);
  setActive(nav.activeId);
  return { setActive };
}
