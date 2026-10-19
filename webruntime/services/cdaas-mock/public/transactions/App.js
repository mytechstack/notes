// Stub experience — proves the left nav is a real, independently loaded
// module per section (same pattern as overview/App.js), not a fully built
// feature. A real build would replace this with the full Transactions
// experience (search, filters, export, etc).

export function mount(el, ctx) {
  el.innerHTML = `
    <div class="experience-card">
      <h2>Transactions</h2>
      <p>This is where the full, searchable transaction history for
      <code>${ctx.tenant}</code> would live. The Home dashboard only shows a
      recent-activity widget — this page would be the complete ledger.</p>
      <p>Module: <code>${ctx.item.experience.scope}/${ctx.item.experience.module}</code>
      · version <code>${ctx.item.experience.moduleVersion}</code></p>
    </div>`;
}
