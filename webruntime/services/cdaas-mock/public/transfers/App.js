// Stub experience — proves the left nav is a real, independently loaded
// module per section (same pattern as overview/App.js), not a fully built
// feature. A real build would replace this with the full Transfers
// experience (wire/ACH, recipients, scheduling, etc).

export function mount(el, ctx) {
  el.innerHTML = `
    <div class="experience-card">
      <h2>Transfers</h2>
      <p>This is where sending/receiving money, recipients, and scheduled
      transfers for <code>${ctx.tenant}</code> would live.</p>
      <p>Module: <code>${ctx.item.experience.scope}/${ctx.item.experience.module}</code>
      · version <code>${ctx.item.experience.moduleVersion}</code></p>
    </div>`;
}
