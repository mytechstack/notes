// Home dashboard v1.1.0 — a separately released module version (the
// catalog lists both 1.0.0 and 1.1.0; each tenant pins one). Adds a
// "Runway" KPI (cash ÷ monthly burn) and escapes every data value before
// it reaches innerHTML.

const fmtUSD = (n) =>
  n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 });

const esc = (v) =>
  String(v).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

function trendHTML(pct) {
  const dir = pct >= 0 ? 'up' : 'down';
  const arrow = pct >= 0 ? '▲' : '▼';
  return `<span class="kpi-trend ${dir}">${arrow} ${esc(Math.abs(pct))}% vs last month</span>`;
}

function render(el, data) {
  const { moneyIn, moneyOut, accounts, transactions, burnRate } = data;
  const burnPct = Math.min(Math.round((burnRate.spent / burnRate.budget) * 100), 100);
  const cash = accounts.reduce((sum, a) => sum + Math.max(a.balance, 0), 0);
  const runwayMonths = burnRate.spent > 0 ? cash / burnRate.spent : Infinity;
  const runwayClass = runwayMonths >= 18 ? 'up' : 'down';

  el.innerHTML = `
    <div class="kpi-row">
      <div class="kpi-card">
        <div class="kpi-label">Money In · ${esc(moneyIn.period)}</div>
        <div class="kpi-amount">${esc(fmtUSD(moneyIn.amount))}</div>
        ${trendHTML(moneyIn.trendPct)}
      </div>
      <div class="kpi-card">
        <div class="kpi-label">Money Out · ${esc(moneyOut.period)}</div>
        <div class="kpi-amount">${esc(fmtUSD(moneyOut.amount))}</div>
        ${trendHTML(moneyOut.trendPct)}
      </div>
      <div class="kpi-card">
        <div class="kpi-label">Runway <span class="kpi-new">new in 1.1</span></div>
        <div class="kpi-amount">${Number.isFinite(runwayMonths) ? esc(runwayMonths.toFixed(1)) + ' months' : '∞'}</div>
        <span class="kpi-trend ${runwayClass}">${esc(fmtUSD(cash))} cash at ${esc(fmtUSD(burnRate.spent))}/month burn</span>
      </div>
    </div>
    <div class="widget-grid">
      <div class="widget-card">
        <h3>Accounts</h3>
        <ul>
          ${accounts.map((a) => `<li><span>${esc(a.name)}</span><span class="amount ${a.balance < 0 ? 'negative' : 'positive'}">${esc(fmtUSD(a.balance))}</span></li>`).join('')}
        </ul>
      </div>
      <div class="widget-card">
        <h3>Recent Transactions</h3>
        <ul>
          ${transactions.map((t) => `<li><span>${esc(t.merchant)}<br><span class="txn-meta">${esc(t.date)} · ${esc(t.category)}</span></span><span class="amount ${t.amount < 0 ? 'negative' : 'positive'}">${esc(fmtUSD(t.amount))}</span></li>`).join('')}
        </ul>
      </div>
      <div class="widget-card">
        <h3>Burn Rate</h3>
        <div class="burn-bar-track"><div class="burn-bar-fill" style="width:${burnPct}%"></div></div>
        <div class="burn-bar-label">${esc(fmtUSD(burnRate.spent))} of ${esc(fmtUSD(burnRate.budget))} monthly budget (${burnPct}%)</div>
      </div>
    </div>`;
}

export async function mount(el, ctx) {
  if (ctx.ssrData) {
    render(el, ctx.ssrData);
    return;
  }
  el.innerHTML = '<p class="loading">Loading dashboard…</p>';
  try {
    const res = await fetch(new URL('../data.json', import.meta.url));
    render(el, await res.json());
  } catch (err) {
    el.innerHTML = `<p class="error">Failed to load dashboard data: ${esc(err)}</p>`;
  }
}
