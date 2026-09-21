The diagram shows the spine of the plan: AI attribution and metrics roll up one level at a time — PR → story → epic → deliverable — with quality and throughput checked at every level, not just the top. Here's the full plan.

## 1. Data model — the attribution chain

Everything downstream depends on getting this join right, in this order:

- **PR → AI flag**: detect via commit co-author trailer (Claude Code adds this automatically) or your agent's session logs matched by repo + timestamp + author.
- **PR → Story**: branch name or PR title contains the Jira key (`ENG-4021-fix-auth`), or Jira's Development panel auto-links it if your GitHub/GitLab integration is on.
- **Story → Epic → Deliverable**: already exists in Jira's hierarchy — no new plumbing needed, just make sure every story is actually linked to a parent epic (orphan stories break the rollup).

**Rollup rule for stories with multiple PRs**: a story is "AI-assisted" if *any* linked PR carries the flag (binary), or you can track a finer-grained `% of PRs AI-assisted` if you want gradient rather than binary attribution. Pick one and stay consistent — mixing the two makes team comparisons meaningless.

**Epic/deliverable AI-intensity** = weighted % of underlying stories (or PRs) that were AI-assisted. This becomes your "how AI-saturated was this deliverable" number.

## 2. Lock estimates before attribution exists

This is the part most teams get wrong: if someone can see a story is going to be AI-assisted *before* they size it, they'll unconsciously (or consciously) size it smaller — and now your "AI improved say-do ratio" result is partly just estimation gaming, not real signal.

- Capture **story point estimate** and **epic/deliverable T-shirt size** (XS/S/M/L/XL) at planning time, before work starts, and treat that field as immutable once sprint planning closes.
- Don't let the AI-assisted flag influence estimation. It's determined after the fact from commits, not decided up front.

## 3. Metrics per layer

| Layer | Metric | Formula |
|---|---|---|
| PR | AI-assisted | commit trailer / session match |
| PR | Cycle time | merged − opened |
| PR | Quality | review comments, iterations, revert flag |
| Story | Cycle time | Done − In Progress (from Jira changelog) |
| Sprint | Say-do ratio | points completed ÷ points committed |
| Sprint | Velocity | total points completed |
| Epic | Cycle time | last story Done − first story In Progress |
| Epic | AI-intensity | % of stories AI-assisted |
| Deliverable | Size delta | estimated T-shirt size − actual (mapped to a day-range table, e.g. XS=1-2d…XL=1-2mo) |
| Deliverable | Throughput | deliverables shipped / quarter |
| Deliverable | Quality | escaped defects, incidents, revert rate tied to its epics |

## 4. The comparison that actually proves impact

Your 5-day-estimated-delivered-in-2-days example is compelling, but a single ticket is an anecdote. To make it a metric:

```
AI-attributable delta = 
    avg(estimate − actual) for AI-assisted work
  − avg(estimate − actual) for non-assisted work, same period
```

This nets out the "we always pad estimates a bit" bias that exists with or without AI, and isolates what AI actually added. Run this at the story level (story points vs. actual cycle time) and at the deliverable level (T-shirt size vs. actual delivery time) — the deliverable-level version is the one your PM/leadership audience will care most about, since "XL became M" is a much more intuitive story than a percentage.

Do the same net-out for velocity: your 40-points-without-AI baseline is exactly the right control. Track velocity for sprints above vs. below a given AI-assist threshold (e.g. >50% of completed stories AI-assisted) rather than just eyeballing "did velocity go up after we adopted AI" — team composition, backlog mix, and seasonality all move velocity too.

## 5. Quality gate — don't let speed hide defects

Throughput gains mean nothing if quality drops, so track these side-by-side with every speed metric, split the same way (AI-assisted vs not):

- **Escaped defect rate** — bugs found after merge, per story
- **Revert/rollback rate** — per PR and per epic
- **Review depth** — comments and iterations per PR (a low number on AI-assisted PRs can mean genuinely clean code, *or* rubber-stamped review — worth spot-checking)
- **Code churn** — % of AI-generated code rewritten within 30/60 days (a leading indicator of quality even before a defect surfaces)
- **Production incidents** — traced back to the deliverable/epic that introduced them

**Rule of thumb for reporting**: only claim a throughput win as "real" if the AI-assisted cohort's defect/revert rate is at or below the non-assisted baseline. If quality is worse, report both numbers honestly — faster-but-buggier isn't the win you're trying to show.

## 6. Rollout phases

1. **Instrument** — PR↔ticket linking, AI-flag detection, lock the estimate field. (This is what the pipeline script from earlier does.)
2. **Baseline** — pull the last several sprints pre-AI: velocity, say-do ratio, cycle time, defect rate, by issue type and size. This is your control group going forward.
3. **Track ongoing** — every sprint, split every metric AI-assisted vs not, with the baseline as the constant reference point.
4. **Roll up to deliverables** — T-shirt size estimate vs. actual, AI-intensity score, quarterly throughput trend.
5. **Report with the quality gate attached** — never show a speed/throughput number without its paired quality number next to it.

**One governance note**: don't turn AI-intensity or say-do ratio into an individual performance metric. The moment it's something devs are personally measured against, you'll get gamed attribution (mis-tagging trivial AI touches, inflating original estimates) and the whole dataset degrades. Keep it team/process-level.

Want me to extend the dashboard with a "deliverable rollup" view — T-shirt size estimate vs. actual, AI-intensity %, and the quality gate side-by-side — so this plan has a visual home alongside the panels we've already built?