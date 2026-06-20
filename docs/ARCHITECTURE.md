# Architecture

## 30-second summary

Signal Garden is a single Go process with explicit internal boundaries. It serves a public portfolio, schedules bounded feed collection, normalizes entries, adds deterministic tags, optionally calls Ollama, persists items atomically, and ranks private search results with lexical and semantic signals.

The first version optimizes for inspectability and recovery rather than scale. Every optional dependency can fail without taking down the public site.

## Components

### `internal/config`

Loads runtime environment settings and the two editable JSON files. It validates required fields and rejects duplicate feed IDs.

### `internal/feed`

Fetches HTTP feeds with timeouts, redirect limits, response-size limits, and a clear user agent. It parses:

- RSS 2.0
- RSS 1.0/RDF
- Atom
- YouTube channel feeds, which are Atom

All incoming HTML is stripped and decoded before storage.

### `internal/collector`

Runs enabled sources through a bounded worker pool. A failed source returns a source-level error and does not cancel other workers. The pipeline is:

```text
fetch -> parse -> normalize -> tag -> optional summarize -> optional embed -> upsert
```

Collection status is stored in memory and broadcast through the SSE hub.

### `internal/tagger`

Applies a deterministic taxonomy based on source defaults and text keywords. This keeps tagging useful when no model is available and makes behavior easy to test.

### `internal/ai`

A small Ollama HTTP client. It uses:

- `/api/chat` for optional short summaries
- `/api/embed` for optional embedding batches and query vectors

AI failures are logged at debug level and the pipeline falls back to source summaries and lexical ranking.

### `internal/store`

The v0 repository is an in-memory map backed by an atomically replaced JSON array. Writes go to a temporary file, call `Sync`, close, and rename. The repository keeps a configured maximum number of newest items.

This is appropriate for a personal single-writer site. It is not the long-term store for a large index.

### `internal/search`

Search builds a small in-memory weighted document view:

- title weight: 4
- tag weight: 5
- summary weight: 2
- content weight: 1
- source name weight: 2

It computes BM25, then adds exact phrase/title/tag boosts. Japanese text adds character bigrams so queries such as `分散` can match longer unsegmented phrases.

When an Ollama query vector and item vectors exist, ranking uses:

```text
0.68 * normalized lexical
+ 0.27 * semantic similarity
+ 0.05 * recency
```

Without embeddings:

```text
0.94 * normalized lexical
+ 0.06 * recency
```

An empty query sorts by exponential recency decay.

### `internal/web`

Uses `net/http`, `html/template`, and embedded static assets. Public and private boundaries are separate:

- Public portfolio: always available
- Inbox and private APIs: optional token cookie
- Refresh endpoint: separate bearer token
- SSE: public clients receive viewer counts; authenticated clients also receive collection events

## Data model

`Item` contains source identity, title, canonical candidate URL, author, cleaned summary/content, timestamps, tags, and an optional vector.

Stable IDs are the first 128 bits of SHA-256 over:

```text
source_id + NUL + source_guid_or_url_or_title
```

This prevents duplicates across refreshes while allowing the same article to exist in two sources until cross-source canonical deduplication is implemented.

## Failure behavior

| Failure | Behavior |
|---|---|
| One source times out | Record source error; continue other sources |
| Feed XML is malformed | Record source error; retain previous items |
| Ollama is offline | Keep source summary; use lexical search |
| Persistence write fails | Return collection error; existing file remains intact |
| Browser JavaScript is disabled | Portfolio, inbox search, and navigation still work |
| SSE disconnects | Browser reconnects automatically; core pages are unaffected |

## Security boundaries

- Feed content is converted to plain text.
- `html/template` performs contextual escaping.
- CSP blocks third-party scripts and framing.
- Inbox and admin tokens are independent.
- Cookies are HttpOnly, SameSite=Lax, and Secure behind HTTPS.
- Robots rules exclude the private index and APIs.
- The sitemap contains only the public root.

## Scaling path

The first scaling step is SQLite, not microservices. Introduce a repository interface, migrations, WAL, normalized source status, and FTS5. Keep collection and web serving in one process until observed load or failure isolation requires otherwise.

A service split becomes reasonable when transcript processing, embedding throughput, or Jetstream consumption has different resource and deployment needs from the public site.
