export function mount(el, ctx) {
  el.innerHTML = `
    <div class="experience-card">
      <h2>Wealth Dashboard</h2>
      <p>Rendered by an experience module loaded from
      <strong>CDAaS mock</strong> for tenant <code>${ctx.tenant}</code>.</p>
      <p>Module: <code>${ctx.item.experience.scope}/${ctx.item.experience.module}</code>
      · version <code>${ctx.item.experience.moduleVersion}</code></p>
      <p>Same Web Runtime shell, same Web Runtime Service, completely
      different experience and permission set — proves tenant isolation
      (RFC capability #1: "multi-tenant config per LOB").</p>
    </div>`;
}
