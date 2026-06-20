# Language decision

Date: 2026-06-20

## Decision

Build the first complete version in Go.

Elixir remains an excellent architectural fit. Gleam remains the most interesting typed BEAM alternative. Neither is a strict upgrade for this project's current priority: deliver a complete, easy-to-run portfolio and personal index that can be handed to another local coding agent with little environment setup.

## Decision matrix

| Criterion | Go | Elixir + Phoenix LiveView | Gleam |
|---|---:|---:|---:|
| One deployable artifact | Excellent | Good | Good |
| RSS and HTTP work | Excellent | Excellent | Good |
| Long-running concurrency | Excellent | Excellent | Excellent on BEAM |
| Real-time server UI | Manual SSE | Excellent | Improving, smaller ecosystem |
| Background-job ecosystem | Manual or libraries | Excellent | Smaller |
| Local AI HTTP integration | Straightforward | Straightforward | Straightforward |
| Database/search ecosystem | Broad | Broad | Thinner |
| Type safety | Strong | Dynamic with specs | Strongest of the three |
| Handoff simplicity | Excellent | Good after BEAM setup | Medium |
| Novelty | Medium | High | Very high |
| Google-oriented portfolio story | Strong systems/concurrency story | Strong distributed/runtime story | Interesting language story |

## Why Go won for v0

1. The entire application builds with the standard library and produces one binary.
2. Feed collection, HTTP serving, SSE, local persistence, and search are directly visible rather than hidden behind a large framework.
3. The architecture creates useful interview material: bounded concurrency, atomic persistence, fallbacks, ranking, authentication boundaries, and graceful degradation.
4. The AT Protocol publishes a reference Go SDK, so a later Bluesky/Jetstream integration has an official path.
5. A local coding agent can build and test the project immediately with `go test ./...` and no service dependencies.

## Why not Elixir for this version

Phoenix LiveView would reduce UI plumbing and OTP is a natural fit for collectors, schedulers, and long-lived connections. It is still the best choice if the project evolves into a multi-node, high-concurrency real-time product.

For the present scope, it would add BEAM/Phoenix environment setup and framework-generated surface area before the product assumptions are validated. The Go implementation keeps the domain boundaries portable; a later Phoenix version can reuse the same feed, item, search, and privacy concepts.

## Why not Gleam for the whole application

Gleam provides static types on the BEAM and is active. It is a good candidate for a focused service or an experimental second implementation. The current ecosystem for mature job scheduling, database adapters, admin tooling, and end-to-end web patterns is smaller than Phoenix's. Using it for every component would optimize novelty before delivery.

## Revisit conditions

Reconsider Elixir when two or more are true:

- many persistent WebSocket or Jetstream connections are required;
- collection is split across nodes;
- per-source supervision and restart isolation become important;
- the UI needs many server-driven real-time interactions;
- job retries, uniqueness, and scheduling outgrow the current collector.

Reconsider Gleam for a contained component when static modeling has clear value and the required libraries are mature.

## Official references checked

- Go release history: https://go.dev/doc/devel/release
- Phoenix documentation: https://hexdocs.pm/phoenix/
- Phoenix LiveView documentation: https://hexdocs.pm/phoenix_live_view/
- Gleam roadmap and releases: https://gleam.run/roadmap/
- AT Protocol SDKs: https://atproto.com/sdks
