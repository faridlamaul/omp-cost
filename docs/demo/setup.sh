#!/usr/bin/env bash
# Creates a throwaway HOME with two Oh My Pi profiles (default, work) whose
# stats.db holds made-up October 2026 usage. Used by demo.tape.
# Usage: docs/demo/setup.sh <dir>
set -euo pipefail

dir="${1:?usage: setup.sh <dir>}"
rm -rf "$dir"
mkdir -p "$dir/.omp/profiles/work"

python3 - "$dir" <<'PY'
import random, sqlite3, sys
from datetime import datetime, timezone

home = sys.argv[1]
# provider, model, $/M input, $/M output, $/M cache read, $/M cache write, tok/s, ttft ms
models = [
    ("anthropic", "claude-opus-5-5", 5.0, 25.0, 0.50, 6.25, 62, 2600),
    ("anthropic", "claude-sonnet-5-5", 3.0, 15.0, 0.30, 3.75, 84, 1900),
    ("openai-codex", "gpt-5.6", 1.25, 10.0, 0.125, 0.0, 41, 4800),
    ("google-antigravity", "gemini-3.8-flash", 0.30, 2.50, 0.03, 0.0, 115, 1300),
]
profiles = {
    "work": (2600, ["-Users-ada-acme-api", "-Users-ada-acme-web", "-Users-ada-acme-infra"], [0.55, 0.3, 0.15], [0.55, 0.25, 0.12, 0.08]),
    "default": (900, ["-Users-ada-oss-rocket", "-Users-ada-oss-notes", "-Users-ada-oss-blog"], [0.5, 0.3, 0.2], [0.2, 0.45, 0.1, 0.25]),
}
start = int(datetime(2026, 10, 1, 8, tzinfo=timezone.utc).timestamp() * 1000)
span = 21 * 24 * 3600 * 1000  # Oct 1 to Oct 22
rng = random.Random(42)  # fixed seed: re-recordings match
# Folders use omp's encoding of the working directory (/ becomes -).

for name, (n, folders, fw, mw) in profiles.items():
    path = f"{home}/.omp/stats.db" if name == "default" else f"{home}/.omp/profiles/{name}/stats.db"
    db = sqlite3.connect(path)
    db.execute("""CREATE TABLE messages (
        id INTEGER PRIMARY KEY, timestamp INTEGER, provider TEXT, model TEXT, folder TEXT,
        agent_type TEXT, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER,
        cache_write_tokens INTEGER, total_tokens INTEGER, cost_total REAL,
        cost_no_cache_input REAL, duration REAL, ttft REAL)""")
    rows = []
    for _ in range(n):
        prov, mdl, pin, pout, pcr, pcw, tps, ttft = rng.choices(models, mw)[0]
        agent = rng.choices(["main", "subagent", "advisor"], [0.75, 0.2, 0.05])[0]
        inp = rng.randint(800, 6000)
        out = rng.randint(300, 2500)
        cr = rng.randint(20000, 140000)
        cw = rng.randint(0, 9000)
        cost = (inp * pin + out * pout + cr * pcr + cw * pcw) / 1e6
        no_cache = ((inp + cr + cw) * pin + out * pout) / 1e6
        t = rng.uniform(0.85, 1.15)
        rows.append((start + rng.randint(0, span), prov, mdl, rng.choices(folders, fw)[0],
                     "" if agent == "main" else agent, inp, out, cr, cw, inp + out + cr + cw,
                     cost, no_cache, ttft * t + out / tps * 1000, ttft * t))
    db.executemany("""INSERT INTO messages (timestamp, provider, model, folder, agent_type,
        input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, total_tokens,
        cost_total, cost_no_cache_input, duration, ttft) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)""", rows)
    db.commit()
    db.close()
PY
