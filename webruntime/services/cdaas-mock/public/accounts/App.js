// Stub experience — proves the left nav is a real, independently loaded
// module per section (same pattern as overview/App.js), not a fully built
// feature. A real build would replace this with the Accounts experience.

export function mount(el, ctx) {
  el.innerHTML = `
    <div class="experience-card">
      <h2>Accounts</h2>
      <p>This is where account details, statements, and account management
      for <code>${ctx.tenant}</code> would live.</p>
      <p>Module: <code>${ctx.item.experience.scope}/${ctx.item.experience.module}</code>
      · version <code>${ctx.item.experience.moduleVersion}</code></p>
    </div>`;
}
