# Signal Garden

A personal portfolio and private technical-signal index in one small Go application.

The public side is a terminal-style portfolio. The private side collects RSS, Atom, and YouTube channel feeds, adds deterministic tags, stores entries locally, and searches them with BM25-style lexical ranking. Ollama summaries and embeddings are optional; the application remains useful when no local model is running.

## Why this exists

The project solves two related problems:

1. Present a small number of shipped projects with the architecture decisions behind them.
2. Reduce information overload from official cloud blogs, engineering publications, newsletters, and video channels.

It intentionally starts as a single process and a single persisted JSON file. The boundaries are separated so the store can later move to SQLite or PostgreSQL without changing feed parsing, tagging, ranking, or HTTP behavior.

## Included

- Public portfolio with a responsive CRT/terminal visual style
- Config-driven project cards and profile content
- RSS 2.0, RSS 1.0/RDF, Atom, and YouTube RSS ingestion
- Concurrent source refresh with per-source status and graceful failures
- Stable IDs, deduplication, atomic local persistence, and retention limits
- Deterministic technical taxonomy
- Weighted BM25 lexical search with Japanese bigram support
- Optional Ollama summaries and semantic embeddings
- Hybrid lexical/semantic ranking with lexical fallback
- Optional token protection for the private inbox
- Bearer-token protected manual refresh API
- Server-Sent Events for viewer count and collection state
- Security headers, health endpoint, robots rules, and sitemap support
- Unit tests, smoke test, Dockerfile, Compose file, and GitHub Actions CI

## Quick start

Requires Go 1.23 or newer. The Docker and CI configuration use Go 1.26.

```bash
cp .env.example .env
# Edit config/site.json and the tokens in .env.
set -a && . ./.env && set +a
go run ./cmd/server
```

Open:

- Portfolio: `http://localhost:8080/`
- Private signal index: `http://localhost:8080/inbox`
- Health: `http://localhost:8080/healthz`

For an offline preview with sample search data:

```bash
make demo-data
INITIAL_REFRESH=false go run ./cmd/server
```

Run all checks:

```bash
make check
make smoke
```

## Trigger a collection manually

```bash
export ADMIN_TOKEN='the-token-from-your-env-file'
make refresh
```

Equivalent request:

```bash
curl -X POST http://localhost:8080/api/refresh \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Optional local AI

Lexical search and rule-based tags need no model. To add local embeddings:

```bash
ollama pull embeddinggemma
export ENABLE_EMBEDDINGS=true
export OLLAMA_EMBED_MODEL=embeddinggemma
go run ./cmd/server
```

To add short summaries, set `OLLAMA_CHAT_MODEL` to any chat model installed in your Ollama instance and enable `ENABLE_LLM_SUMMARY=true`.

The application uses Ollama's native `/api/embed` and `/api/chat` endpoints. If Ollama is unavailable, collection and search continue without semantic features.

## Configuration

- `config/site.json`: profile, links, skills, and selected projects
- `config/feeds.json`: source URL, source type, default tags, and enabled state
- `data/items.json`: local persisted index
- `.env`: runtime secrets and operational settings

YouTube channels expose Atom feeds in this form:

```text
https://www.youtube.com/feeds/videos.xml?channel_id=CHANNEL_ID
```

The Google Cloud Tech example is included but disabled until you confirm the channel ID and desired volume.

## API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Liveness and item count |
| `GET` | `/api/search?q=...` | Authenticated JSON search |
| `GET` | `/api/status` | Authenticated collection status |
| `POST` | `/api/refresh` | Bearer-token protected refresh |
| `GET` | `/events` | SSE viewer count; private status when authenticated |

## Deployment

```bash
cp .env.example .env
# Set strong INBOX_TOKEN and ADMIN_TOKEN values.
docker compose up --build -d
```

The container runs as a non-root user. Persist `data/` and back up `data/items.json`. Set `PUBLIC_BASE_URL` to expose a sitemap.

## Project documents

- [LANGUAGE_DECISION.md](LANGUAGE_DECISION.md): why this implementation uses Go instead of Elixir or Gleam
- [HANDOFF.md](HANDOFF.md): current state, invariants, gaps, and prioritized continuation plan
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): components and data flow
- [docs/OPERATIONS.md](docs/OPERATIONS.md): security, backup, deploy, and recovery
- [docs/NEXT_PROMPTS.md](docs/NEXT_PROMPTS.md): focused prompts for local Claude Code
- [CLAUDE.md](CLAUDE.md): repository instructions for coding agents

## License

MIT
