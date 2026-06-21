# Hjosugi Hub

An Elixir-powered static portfolio and public technical radar.

The deployed site is plain static HTML/CSS/JS for GitHub Pages. Elixir Mix tasks
collect RSS/Atom feeds, normalize and tag items, then export the portfolio and
searchable radar page to `public/`.

## Quick Start

Requires Elixir 1.16 or newer.

```bash
mix test
mix hub.collect
mix hub.export --out public
```

Open `public/index.html` for the portfolio and `public/radar/index.html` for
the searchable radar index. For the radar page, serve `public/` over HTTP so
browser `fetch()` can load `data/items.json`.

For example:

```bash
python3 -m http.server 4000 --directory public
```

Then open `http://localhost:4000/`.

## Configuration

- `config/site.exs`: profile, links, skills, and selected projects
- `config/feeds.exs`: RSS/Atom/YouTube feed sources
- `priv/static_site/templates/`: static HTML templates
- `priv/static_site/assets/`: CSS and browser-side search JavaScript

## Deployment

GitHub Pages is the intended production target for Hjosugi Hub: no always-on
server, no database, and no paid worker. The repository should be named
`hjosugi-hub`; with the default GitHub Pages domain the site will be published at
`https://hjosugi.github.io/hjosugi-hub/`.

Deployment is handled by [`.github/workflows/pages.yml`](.github/workflows/pages.yml):

1. `mix test`
2. `mix hub.collect --timeout 20000 --workers 6 --max-items 1000`
3. `mix hub.export --out public --base-url <GitHub Pages URL>`
4. Deploys `public/` with `actions/deploy-pages`

Set it up once in GitHub:

1. Open the repository settings for `hjosugi/hjosugi-hub`.
2. Go to **Pages**.
3. Set **Build and deployment** -> **Source** to **GitHub Actions**.
4. Push to `main`, or run **Deploy Hjosugi Hub** from the Actions tab.

The deploy workflow also runs every six hours to refresh the public radar data.
It passes GitHub Pages' resolved URL to `mix hub.export --base-url`, so
`robots.txt` and `sitemap.xml` match the actual Pages URL. No secrets are
required.

`public/` is generated output and is intentionally ignored by Git. Do not commit
it; GitHub Pages receives it from the workflow artifact.

Important: GitHub Pages cannot hide collected data. Anything in
`public/data/items.json` is public.

## License

MIT
