# Repository instructions for Claude Code

## Start here

Read, in order:

1. `HANDOFF.md`
2. `LANGUAGE_DECISION.md`
3. `docs/ARCHITECTURE.md`
4. `docs/OPERATIONS.md`

Then run:

```bash
make check
make smoke
```

Do not begin feature work if the baseline is failing.

## Engineering rules

- Keep the public portfolio functional without JavaScript, network access, or Ollama.
- Keep lexical search as a complete fallback.
- Do not render feed HTML directly.
- Preserve bounded concurrency and source-level failure isolation.
- Keep secrets in environment variables, never config JSON or templates.
- Prefer Go's standard library. Before adding a dependency, document why the standard library is insufficient.
- Add tests for parsers, ranking, persistence, and authentication changes.
- Do not silently change project claims in `config/site.json`; verify them against the linked repository.
- Use small vertical commits. Update `HANDOFF.md` after finishing a phase.

## Useful commands

```bash
make run
make check
make smoke
make demo-data
```

## Current highest-priority work

1. Conditional feed requests with ETag and Last-Modified persistence
2. Source-health UI
3. Canonical URL and content-hash deduplication
4. SQLite repository adapter
5. Bluesky selected-account ingestion
