"""
ADR index adapter.

Production note: this does naive keyword scoring over local markdown files.
Swap the body of `get_relevant_adrs` for a call to your real doc-search /
embeddings index — the signature below is the contract callers rely on,
so keep it stable while you upgrade the retrieval quality underneath.
"""
import os
import re
from dataclasses import dataclass

from . import REPO_ROOT

ADR_DIR = os.path.join(REPO_ROOT, "platform", "docs", "adr")


@dataclass
class ADR:
    id: str
    title: str
    excerpt: str


# Generic engineering nouns that appear in almost every ticket/ADR and
# carry no discriminating signal on their own — excluded from scoring so
# they don't cause an unrelated ADR to match on "service" or "api" alone.
_STOPWORDS = {
    "add", "the", "and", "for", "with", "this", "that", "api", "service",
    "services", "app", "application", "new", "our", "into",
}


def _score(text: str, query_terms: list[str]) -> int:
    text_lower = text.lower()
    score = 0
    for term in query_terms:
        # whole-word match only — avoids "service" matching inside
        # "services" being the only signal, and avoids partial-word noise
        score += len(re.findall(rf"\b{re.escape(term)}\b", text_lower))
    return score


def get_relevant_adrs(query: str, top_k: int = 2, min_score: int = 1) -> list[ADR]:
    """
    SELECTION: return only ADRs relevant to `query`, not every ADR in the
    index. `min_score` ensures a low-relevance ADR doesn't get included
    just to fill top_k — keeps irrelevant history (e.g. logging
    conventions) out of context entirely, rather than relying on the
    model to ignore it.
    """
    query_terms = [
        t for t in re.findall(r"[a-z]+", query.lower())
        if len(t) > 2 and t not in _STOPWORDS
    ]

    candidates = []
    for filename in sorted(os.listdir(ADR_DIR)):
        if not filename.endswith(".md"):
            continue
        path = os.path.join(ADR_DIR, filename)
        with open(path) as f:
            content = f.read()

        score = _score(content, query_terms)
        if score < min_score:
            continue

        # Pull just the ID + title (first line) and the Decision section —
        # COMPRESSION: not the full ADR (context, consequences, etc.)
        adr_id = filename.split("-")[0] + "-" + filename.split("-")[1]
        title_line = content.splitlines()[0].lstrip("# ").strip()
        # strip a redundant leading "ADR-142: " if the file's own title
        # line repeats the id, so callers don't double-print it
        title_line = re.sub(rf"^{re.escape(adr_id)}:\s*", "", title_line)
        decision = _extract_section(content, "Decision")

        candidates.append((score, ADR(id=adr_id, title=title_line, excerpt=decision)))

    candidates.sort(key=lambda pair: pair[0], reverse=True)
    return [adr for _, adr in candidates[:top_k]]


def _extract_section(content: str, heading: str) -> str:
    lines = content.splitlines()
    capture = False
    out = []
    for line in lines:
        if line.strip().startswith("##"):
            capture = heading.lower() in line.lower()
            continue
        if capture and line.strip():
            out.append(line.strip())
    return " ".join(out)
