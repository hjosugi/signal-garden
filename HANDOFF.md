# Signal Garden handoff

## Product goal

Ship one small site with two boundaries:

- Public: a clear engineering portfolio centered on a few finished projects and their design decisions.
- Private: a personal technical-signal index that collects trusted sources and makes them searchable without visiting every platform.

The visual reference is a terminal/CRT portfolio, but the implementation and copy are original.

## Current implementation status

Working now:

- Config-driven portfolio with Daimon, Mail Lookout, open-wiki, and YouTube Tools
- Responsive terminal UI with no external font or JavaScript framework
- RSS 2.0, RDF/RSS 1.0, Atom, and YouTube-feed parsing
- Concurrent refresh with bounded workers and source-level errors
- Stable entry IDs and cross-run deduplication
- Atomic JSON-file persistence and retention cap
- Deterministic taxonomy
- Weighted BM25 search, phrase boosts, recency, and Japanese bigrams
- Optional Ollama summaries and embeddings
- Hybrid ranking with automatic lexical fallback
- Optional Inbox token, separate admin refresh token, secure headers
- SSE viewer count and private collection events
- Tests, smoke test, Docker, Compose, CI, and documentation

Run verification:

```bash
make check
make smoke
```

## Important invariants

Do not break these without writing an ADR:

1. The public portfolio must work when every feed and AI service is unavailable.
2. Lexical search must remain the fallback when Ollama is absent or slow.
3. A source failure must not cancel other source jobs.
4. Persisted item replacement must remain atomic.
5. `ADMIN_TOKEN` and `INBOX_TOKEN` have separate responsibilities.
6. Feed HTML is converted to text and never rendered as trusted HTML.
7. New dependencies need a concrete benefit and a maintenance note.
8. The public page must remain readable with JavaScript disabled.
9. The private index must stay excluded from robots and public sitemap output.
10. Project claims in `config/site.json` must match the repositories.

## Architecture map

```text
config/feeds.json
       |
       v
bounded collector workers ---> RSS/Atom fetch + parse
       |                              |
       |                              v
       |                         normalized Item
       |                              |
       +--> deterministic tags -------+
       +--> optional Ollama summary ---+
       +--> optional embedding --------+
                                      |
                                      v
                           atomic JSON repository
                                      |
                   +------------------+------------------+
                   |                                     |
                   v                                     v
          BM25 + semantic rank                     portfolio/inbox UI
                   |                                     |
                   +---------------- SSE status ---------+
```

## Known limitations

- The JSON repository rewrites the full file after changes. It is intentionally simple and should move to SQLite before the index becomes large or multi-writer.
- HTTP cache validators (`ETag`, `Last-Modified`) are not stored, so unchanged feeds are downloaded again.
- Feed health is visible through `/api/status` but not yet presented as a dedicated admin table.
- LLM summaries are limited per source per run but do not use a durable retry queue.
- Cross-source duplicate detection currently uses source-specific stable IDs. Canonical-URL and content-hash deduplication are not implemented.
- Bluesky is documented but not implemented. Use public author feeds for a simple first step or the official Go SDK/Jetstream for real-time ingestion.
- The Inbox authentication is appropriate for a personal site behind HTTPS, not for a multi-user product.
- JSON persistence is a system of record and search input in v0; there is no separate rebuildable index file yet.

## Prioritized continuation plan

### P0: personalize and publish

- Edit `config/site.json` copy, contact links, and availability.
- Create the repository and add Signal Garden itself after deployment.
- Verify every enabled feed from the deployment network.
- Set strong, different `INBOX_TOKEN` and `ADMIN_TOKEN` values.
- Deploy behind HTTPS and set `PUBLIC_BASE_URL`.
- Add the custom domain, then use it as the Bluesky handle.

### P1: operational correctness

- Persist `ETag` and `Last-Modified` per source and send conditional requests.
- Add exponential backoff and a failure threshold for noisy sources.
- Add a private source-health table with last success, last error, latency, and item count.
- Add canonical URL normalization and content-hash duplicate detection.
- Add store backup and restore commands.

### P2: SQLite repository

Introduce a repository interface and a SQLite implementation with:

- `items`, `sources`, `tags`, `item_tags`, and `collection_runs` tables;
- WAL mode and migrations;
- FTS5 for lexical retrieval;
- atomic source/item/status transactions;
- a JSON export command for portability.

Keep the current JSON repository as a test and demo adapter.

### P3: better retrieval

- Persist embedding model name and dimensions with every vector.
- Re-embed only when model/version changes.
- Fuse FTS and semantic ranks with Reciprocal Rank Fusion.
- Add saved searches and a daily digest.
- Add explicit feedback: useful, noisy, already known.

### P4: Bluesky and video enrichment

- Start with selected public author feeds and backfill.
- Move to Jetstream only when near-real-time updates are useful.
- Store Bluesky DID and post URI, not only handles.
- For YouTube, store channel ID and video ID; optionally fetch transcripts in a separate bounded worker.

## Local Claude Code kickoff

Give Claude Code this prompt from the repository root:

```text
Read CLAUDE.md, HANDOFF.md, LANGUAGE_DECISION.md, and docs/ARCHITECTURE.md.
Run `make check` and `make smoke` before editing.
Do not add dependencies yet.
Review config/site.json for portfolio clarity and config/feeds.json for malformed or duplicate entries.
Then implement the highest-value P1 item: conditional RSS requests using persisted ETag and Last-Modified metadata.
Add tests for 200, 304, changed ETag, and source failure. Preserve all invariants in HANDOFF.md.
```

## Acceptance criteria for the next owner

Before merging any major change:

```bash
make check
make smoke
```

Also verify manually:

- `/` loads with JavaScript disabled;
- `/inbox` redirects to `/unlock` when a token is set;
- one broken feed does not stop healthy feeds;
- an unavailable Ollama instance does not break lexical search;
- no raw feed HTML appears in rendered pages;
- restart preserves indexed items.
