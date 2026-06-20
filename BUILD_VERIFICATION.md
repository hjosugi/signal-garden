# Build verification

Verified on 2026-06-20 in the artifact environment with Go 1.23.2.

Commands completed successfully:

```bash
make check
go test -race ./...
go test -cover ./...
make smoke
```

The source targets Go 1.23 syntax and standard-library APIs. Docker and GitHub Actions are configured to build with Go 1.26.x.

The smoke test verifies:

- server startup and `/healthz`;
- public portfolio rendering;
- redirect of a locked Inbox;
- token unlock and authenticated Inbox rendering;
- authenticated JSON search with sample data;
- initial SSE viewer-count event.

Live third-party feed fetching was not executed in the isolated build environment. The application records source-level failures, and feed URLs should be checked from the actual deployment network after launch.
