# Operations

## Production checklist

1. Set unique, high-entropy `INBOX_TOKEN` and `ADMIN_TOKEN` values.
2. Serve the application only through HTTPS.
3. Set `PUBLIC_BASE_URL` to the canonical domain.
4. Mount `data/` on durable storage.
5. Back up `data/items.json` and both JSON config files.
6. Confirm filesystem ownership allows the non-root container user to replace `data/items.json`.
7. Run a manual refresh and inspect `/api/status` before enabling periodic collection.
8. Keep AI features disabled until Ollama model availability and latency are verified.

## Local run

```bash
cp .env.example .env
set -a && . ./.env && set +a
go run ./cmd/server
```

## Container run

```bash
cp .env.example .env
docker compose up --build -d
docker compose logs -f app
```

With the optional Ollama service:

```bash
docker compose --profile ai up -d ollama
# Pull a model inside the Ollama container or from its API.
docker compose up --build -d app
```

## Health and status

Public liveness:

```bash
curl -fsS https://your-domain.example/healthz
```

Private status requires the Inbox session in the browser. For operational automation, use logs and the refresh response; a dedicated machine-authenticated status endpoint is intentionally not included in v0.

Manual refresh:

```bash
curl -fsS -X POST https://your-domain.example/api/refresh \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

A `202` response means a run started. A `409` means one is already active.

## Backup

For v0, pause collection or copy between runs:

```bash
cp data/items.json "backup/items-$(date +%Y%m%d-%H%M%S).json"
cp config/site.json backup/
cp config/feeds.json backup/
```

Restore by replacing `data/items.json` while the process is stopped, then restart and check `/healthz`.

## Source failures

Source errors do not remove existing entries. Common causes:

- feed URL moved;
- anti-bot response instead of XML;
- timeout or DNS failure;
- malformed publisher output;
- response exceeded the 5 MiB safety limit.

Use the authenticated `/api/status` output to find `last_error` and `response_code`. Disable a noisy source in `config/feeds.json` until corrected.

## AI failures

Ollama is optional. If embeddings or summaries fail:

- collection continues;
- source-provided/extractive summaries remain;
- search falls back to BM25;
- no public-page outage occurs.

Use the same embedding model for indexed items and queries. Changing models without re-embedding produces incompatible vectors; the planned SQLite migration should persist model metadata and a reindex state.

## Security notes

- Do not expose the site over plain HTTP outside localhost.
- Do not reuse the Inbox token as the admin token.
- Do not place secrets in `compose.yaml`, JSON config, or Git history.
- The simple Inbox cookie is for one trusted owner, not multiple users or role-based access.
- Put the application behind a platform with request limits and TLS termination.

## Rollback

The binary and content config are independent. Keep the previous binary image tag and a data backup. Roll back the image without rewriting data. If a schema-backed store is added later, every migration must include an explicit rollback or forward-repair procedure.
