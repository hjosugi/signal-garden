# Repository instructions

This project is an Elixir static-site pipeline for GitHub Pages.

## Commands

```bash
mix format --check-formatted
mix test
mix hub.collect
mix hub.export --out public
```

Deployment is handled by `.github/workflows/pages.yml` (`Deploy Hjosugi Hub`).
GitHub Pages source should be set to GitHub Actions for `hjosugi/hjosugi-hub`.
The workflow exports `public/` and deploys it as a Pages artifact; do not commit
`public/`.

## Rules

- Do not reintroduce non-Elixir runtime code.
- Keep the deployed site static and cheap to host.
- Treat everything exported under `public/` as public.
- Convert feed content to plain text before rendering or exporting it.
- Keep `config/site.exs` and `config/feeds.exs` human-editable.
- Avoid dependencies unless OTP/Elixir standard tooling is clearly insufficient.
