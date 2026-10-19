// Home dashboard — the one experience in this POC wired up to
// __EXPERIENCE_DATA__ (see ssrdata.go in the Web Runtime Service and
// runtime.js). When the server already fetched this data, ctx.ssrData is
// populated and mount() renders immediately, no spinner. Otherwise this
// falls back to fetching the same data.json itself, same as any plain
// CSR experience would — it never assumes the server-side fetch happened.

const fmtUSD = (n) =>
  n.toLocaleString('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 });

function trendHTML(pct) {
  const dir = pct >= 0 ? 'up' : 'down';
  const arrow = pct >= 0 ? '▲' : '▼';
  return `<span class="kpi-trend ${dir}">${arrow} ${Math.abs(pct)}% vs last month</span>`;
}

function render(el, data) {
  const { moneyIn, moneyOut, accounts, transactions, burnRate } = data;
  const burnPct = Math.min(Math.round((burnRate.spent / burnRate.budget) * 100), 100);

  el.innerHTML = `
    <div class="kpi-row">
      <div class="kpi-card">
        <div class="kpi-label">Money In · ${moneyIn.period}</div>
        <div class="kpi-amount">${fmtUSD(moneyIn.amount)}</div>
        ${trendHTML(moneyIn.trendPct)}
      </div>
      <div class="kpi-card">
        <div class="kpi-label">Money Out · ${moneyOut.period}</div>
        <div class="kpi-amount">${fmtUSD(moneyOut.amount)}</div>
        ${trendHTML(moneyOut.trendPct)}
      </div>
    </div>
    <div class="widget-grid">
      <div class="widget-card">
        <h3>Accounts</h3>
        <ul>
          ${accounts
            .map(
              (a) => `<li><span>${a.name}</span><span class="amount ${a.balance < 0 ? 'negative' : 'positive'}">${fmtUSD(a.balance)}</span></li>`
            )
            .join('')}
        </ul>
      </div>
      <div class="widget-card">
        <h3>Recent Transactions</h3>
        <ul>
          ${transactions
            .map(
              (t) => `<li><span>${t.merchant}<br><span class="txn-meta">${t.date} · ${t.category}</span></span><span class="amount ${t.amount < 0 ? 'negative' : 'positive'}">${fmtUSD(t.amount)}</span></li>`
            )
            .join('')}
        </ul>
      </div>
      <div class="widget-card">
        <h3>Burn Rate</h3>
        <div class="burn-bar-track"><div class="burn-bar-fill" style="width:${burnPct}%"></div></div>
        <div class="burn-bar-label">${fmtUSD(burnRate.spent)} of ${fmtUSD(burnRate.budget)} monthly budget (${burnPct}%)</div>
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
    const res = await fetch(new URL('./data.json', import.meta.url));
    render(el, await res.json());
  } catch (err) {
    el.innerHTML = `<p class="error">Failed to load dashboard data: ${err}</p>`;
  }
}
