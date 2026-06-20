# Prompts for local Claude Code

Use one prompt at a time. Require tests and a HANDOFF update in every prompt.

## 1. Verify and personalize the portfolio

```text
Read CLAUDE.md and HANDOFF.md. Run `make check` and `make smoke`.
Review config/site.json and the linked public repositories. Tighten every project summary so it states problem, architecture decision, and tradeoff without hype. Do not invent metrics. Keep the public page in simple English suitable for a Google Japan recruiter or engineer. Add tests only if code changes are needed. Update HANDOFF.md with any content decisions.
```

## 2. Conditional RSS requests

```text
Read CLAUDE.md, HANDOFF.md, and docs/ARCHITECTURE.md. Run the baseline checks.
Implement persisted ETag and Last-Modified metadata per source. Send If-None-Match and If-Modified-Since on later requests. Treat HTTP 304 as a successful no-change result. Do not add a dependency. Add tests for first 200, 304, changed validator, malformed feed, and timeout. Preserve source-level failure isolation. Update architecture and handoff documents.
```

## 3. Source-health page

```text
Add an authenticated /inbox/sources page. Show enabled state, source kind, last attempt, last success, response code, items seen, items added, and last error. Keep JavaScript optional. Add links from the Inbox and tests for authentication and HTML escaping. Do not expose this page in robots or sitemap output.
```

## 4. SQLite repository

```text
Design a repository interface that preserves current JSON behavior. Add a SQLite adapter with migrations, WAL, items, sources, tags, item_tags, and collection_runs. Use FTS5 for lexical retrieval. Keep the JSON adapter for tests and offline demo. Write an ADR explaining the dependency choice, transaction boundaries, backup, and rollback. Add migration and parity tests. Do not remove the lexical fallback or optional Ollama behavior.
```

## 5. Bluesky selected-account ingestion

```text
Research current official AT Protocol documentation and Go SDK before coding. Add selected-account ingestion, not the full network. Store DID and AT URI as stable identity. Start with backfilled author feeds; only add Jetstream if it materially improves freshness. Implement cursor/retry behavior, rate limits, and tests with fixture responses. Keep Bluesky failures isolated from RSS collection. Update the feed config schema and handoff docs.
```

## 6. Daily digest

```text
Add a private daily digest generated from the last 24 hours. Rank by source priority, novelty, and search feedback. The first version must be deterministic and work without an LLM. Optionally let Ollama compress the final digest, but retain a fallback. Expose HTML and Markdown export. Add tests for time zones, empty days, duplicates, and unavailable Ollama.
```
