// Stand-in for a CDAaS-delivered experience module (real CDAaS uses Module
// Federation's remoteEntry.js; this POC uses a plain ES module loaded via
// dynamic import() — RFC treats CDAaS as "retained unchanged" so it's
// mocked as a dumb static host here, not reimplemented).

export function mount(el, ctx) {
  el.innerHTML = `
    <div class="experience-card">
      <h2>Overview</h2>
      <p>Rendered by an experience module loaded from
      <strong>CDAaS mock</strong> for tenant <code>${ctx.tenant}</code>.</p>
      <p>Module: <code>${ctx.item.experience.scope}/${ctx.item.experience.module}</code>
      · version <code>${ctx.item.experience.moduleVersion}</code></p>
      <p>This module was fetched by the browser directly from CDAaS —
      the Web Runtime Service never touched it.</p>
    </div>`;
}
