# shortr

**Self-hosted URL shortener and QR code generator.** One container. No accounts, no tracking, every link
expires.

[![Docker Hub](https://img.shields.io/docker/v/dawidzbinski/shortr?sort=semver&label=docker%20hub&color=2b6cff)](https://hub.docker.com/r/dawidzbinski/shortr)
[![Image size](https://img.shields.io/docker/image-size/dawidzbinski/shortr/latest?label=image&color=2b6cff)](https://hub.docker.com/r/dawidzbinski/shortr)
[![License](https://img.shields.io/badge/license-PolyForm%20Noncommercial-2b6cff)](LICENSE)

```sh
docker run -d -p 8080:8080 dawidzbinski/shortr:latest
```

[Install](#install) · [Self-hosting](#self-hosting) · [Configuration](#configuration) · [API](#api) ·
[Client](#client) · [Security](#security) · [Technical](#technical) · [License](#license)

## What it does

- **Shortens** — one URL in, a 12 character code out, `302` to the target.
- **QR codes** — the same short link as a QR, copied to the clipboard as a PNG.
- **Expires** — every link dies after `URL_TTL`, 30 days by default. Nothing to prune.
- **Stays private** — no sign-up, no analytics, no cookies. Everything is `noindex`, crawlers get `403`.
- **Runs anywhere** — a ~15 MB distroless image for `amd64`, `arm64` and `armv7`. Redis optional.

Free for noncommercial use — see [License](#license).

## Install

### Docker

```sh
docker run -d --name shortr -p 8080:8080 dawidzbinski/shortr:latest
```

Open <http://localhost:8080>. Links are held in the process and lost on restart, which is fine for a trial.
For anything real, add Redis.

### Docker Compose, with Redis

```yaml
services:
    shortr:
        image: dawidzbinski/shortr:latest
        restart: unless-stopped
        ports:
            - '8080:8080'
        environment:
            BASE_URL: https://s.example.com
            URL_PERSISTER: redis
            REDIS_HOST: redis
            REDIS_PORT: '6379'
            REDIS_USER: default
            REDIS_PASSWORD: change-me
        depends_on:
            redis:
                condition: service_healthy

    redis:
        image: redis:8-alpine
        restart: unless-stopped
        command: ['redis-server', '--requirepass', 'change-me', '--appendonly', 'yes']
        volumes:
            - redis-data:/data
        healthcheck:
            test: ['CMD', 'redis-cli', '-a', 'change-me', 'ping']
            interval: 5s
            timeout: 3s
            retries: 20

volumes:
    redis-data:
```

```sh
docker compose up -d
```

## Self-hosting

| Concern | What to do |
| --- | --- |
| **Public origin** | Set `BASE_URL` to the URL your users see. Short links and link previews are built from it. |
| **TLS** | Terminate at your proxy. shortr speaks plain HTTP on `PORT`. |
| **Behind a proxy** | Set `TRUSTED_PROXIES` to the proxy's CIDRs. Without it `X-Forwarded-For` is ignored and every client shares one rate-limit bucket. |
| **Health checks** | Probe `GET /healthz` from outside the container — the image has no shell, so in-container checks cannot run. |
| **Persistence** | `URL_PERSISTER=redis`. The memory adapter is per-process, so it also rules out a second replica. |
| **Rate limits** | `RATE_LIMIT_MODE` and `RATE_LIMIT_VALUE`, per client IP, on `/api` only. Counters are per process. |
| **Upgrades** | `docker compose pull && docker compose up -d`. |

### Image tags

| Tag | Moves |
| --- | --- |
| `latest` | on every release |
| `1` | on every release in major 1 |
| `1.2` | on every patch of 1.2 |
| `1.2.3` | never |

Pin `1` to take fixes and features but approve majors yourself.

## Configuration

Every setting is an environment variable read at startup. An invalid value aborts the start with an error
naming the variable; an empty one falls back to the default.

### Core

| Variable | Default | Rules |
| --- | --- | --- |
| `BASE_URL` | `http://localhost:8080` | Public origin for short links and Open Graph tags. `http`/`https` with a host; trailing slashes stripped. |
| `PORT` | `8080` | 1–65535. |
| `URL_TTL` | `30d` | Link lifetime. Go duration syntax (`12h`, `90m`) plus an `Nd` day suffix. Greater than zero. |
| `URL_MAX_LENGTH` | `4096` | Longest target URL in bytes, 256–65536. The body cap follows it, plus 1024 bytes for the JSON wrapper. |
| `URL_PERSISTER` | `memory` | Storage adapter: `memory` or `redis`. Unknown names abort the start. |
| `RATE_LIMIT_MODE` | `day` | Window: `second`, `minute`, `hour` or `day`. |
| `RATE_LIMIT_VALUE` | `30` | Requests per window per client IP. At least 1. |
| `TRUSTED_PROXIES` | *(empty)* | Comma separated CIDRs. Empty means `X-Forwarded-For` is ignored. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. JSON on stdout. |
| `STATIC_DIR` | `./apps/client/dist` | Where the built client lives. The image ships with `/srv/client`; leave it alone. |

### Storage — `memory` *(default)*

No variables. Links live in the process: they vanish on restart and are not shared between replicas.

### Storage — `redis`

Set `URL_PERSISTER=redis`, then:

| Variable | Default | Rules |
| --- | --- | --- |
| `REDIS_HOST` | *(required)* | Non-empty host name. |
| `REDIS_PORT` | *(required)* | 1–65535. |
| `REDIS_USER` | *(required)* | Non-empty user name. |
| `REDIS_PASSWORD` | *(required)* | Non-empty. Kept untrimmed, never logged or echoed in an error. |
| `REDIS_DB` | `0` | 0 or greater. |

Keys are written as `url:<code>` with `SET NX EX`, so Redis owns expiry.

## API

Errors share one shape. The `error` code is stable; the `message` states which rule actually failed.

```json
{ "error": "invalid_url", "message": "the url must be at most 4096 characters" }
```

### `POST /api/shorten`

`Content-Type: application/json`. Rate limited and bot filtered. `201` on success, `expiresAt` is RFC 3339.

```sh
curl -X POST http://localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/a-rather-long-link"}'
```

```json
{
  "code": "aB3xY7kQ1mZp",
  "shortUrl": "http://localhost:8080/aB3xY7kQ1mZp",
  "expiresAt": "2026-10-05T12:00:00Z"
}
```

| Status | `error` | Cause |
| --- | --- | --- |
| 400 | `invalid_body` | Not a JSON object with a non-empty `url`. |
| 400 | `invalid_url` | Not an absolute `http`/`https` URL, over `URL_MAX_LENGTH`, or carrying whitespace, control characters or `user:password@`. |
| 403 | `forbidden` | `User-Agent` matches the blocked crawler list. |
| 413 | `body_too_large` | Over `URL_MAX_LENGTH` + 1024 bytes. |
| 415 | `unsupported_media_type` | Content type is not JSON. |
| 429 | `rate_limited` | Limit exhausted. `Retry-After` on the rejection; `X-RateLimit-*` on accepted requests. |
| 500 | `internal` | Storage failure, or no free code after 5 attempts. |

### `GET /{code}`

`302` to the target with `Cache-Control: no-store`. Neither rate limited nor bot filtered. `404 not_found`
when the code is unknown, expired, or not 12 characters of `[0-9A-Za-z]`.

### `GET /healthz`

`200 {"status":"ok"}` when the adapter answers a ping, `503 unavailable` when it does not. Never bot
filtered, so probes work with any user agent.

### Client files

`/`, `/robots.txt`, `/favicon.svg`, `/favicon.png`, `/apple-touch-icon.png`, `/og.png` and `/assets/*` are
served from `STATIC_DIR` with a one hour `max-age` on assets. `/` gets `BASE_URL` substituted into its Open
Graph tags. All bot filtered.

## Client

One page, two modes — *Shorten URL* and *QR code* — with no build step to run and no external requests at
runtime.

- **Mode** is a tablist stored in `localStorage` under `shortr.mode`. Roving tabindex: Left/Right move,
  Home/End jump to the ends.
- **Result** is copied the moment it appears: the short URL as text, or the QR as a PNG through
  `ClipboardItem`. Clicking it copies again.
- **Expiry** sits under the link, built from `expiresAt` — `Expires in 30 days · 4 Oct 2026`.
- **Errors** render in a `role="alert"` element, status code first. A `429` shows the seconds to wait.
- **Fonts** are self-hosted woff2 under `/assets/`, so nothing is fetched from a CDN.

## Security

- **Codes** are 12 characters of `[0-9A-Za-z]` from `crypto/rand`, drawn without modulo bias — ~71 bits, so
  links cannot be enumerated.
- **Everything expires** after `URL_TTL`. Redis expires its own keys; the memory adapter expires on read and
  sweeps in the background.
- **Not indexable** — `X-Robots-Tag: noindex, nofollow, noarchive` on every response, a `robots.txt` that
  disallows everything, and `403` for known crawler user agents on the client and the API. Redirects and
  `/healthz` stay open.
- **Headers** — strict CSP (`default-src 'self'`, no inline scripts or styles, `frame-ancestors 'none'`,
  `base-uri 'none'`), `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`.
- **Input limits** — URLs capped at `URL_MAX_LENGTH`, bodies at that plus 1024 bytes. URLs with whitespace,
  control characters or embedded credentials are rejected.
- **Rate limiting** is a fixed window per client IP on `/api` only; redirects are never limited. Set
  `TRUSTED_PROXIES` behind a proxy or every client counts as one.

## Technical

A Go 1.27 API on [Fiber v3](https://github.com/gofiber/fiber) serves the JSON endpoints, the redirects and a
plain TypeScript client bundled by esbuild — one binary, one image.

### The image

`.docker/Dockerfile` is a single multi-stage file. The last stage is
[`dawidzbinski/shortr`](https://hub.docker.com/r/dawidzbinski/shortr).

- Base `gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager, no toolchain.
- Contents: the static `/shortr` binary (`CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`) and the client in
  `/srv/client`, source maps stripped.
- `ENV STATIC_DIR=/srv/client PORT=8080`, `EXPOSE 8080`, `USER nonroot`.

### Development

Everything runs in Docker; the host needs only Docker and [Task](https://taskfile.dev).

```sh
task project:start   # api + client watcher + redis, hot reload on http://localhost:8080
task project:stop    # and drop the volumes
```

| Task | What it does |
| --- | --- |
| `task project:lint` | Lint the API, the client and the e2e suite. |
| `task project:test` | API and client unit tests. |
| `task project:e2e` | Playwright suite against the production image. |
| `task api:build` | Build the production image locally as `shortr:latest`. |
| `task client:build` | Bundle the client into `apps/client/dist`. |

`task --list-all` prints the rest. To push to your own registry instead of Docker Hub, copy
`Taskfile.local.example.yml` to `Taskfile.local.yml` (gitignored), set `IMAGE`, and run `task project:deploy`.

The landing page in `docs/` repeats this README and ships to GitHub Pages on every push to `main` that
touches `docs/**` — change the two together.

### Writing an adapter

Storage sits behind one interface. A new backend touches `adapters/<name>.go`, its test, and one case in
`adapters.New`; nothing else learns about it.

```go
type Adapter interface {
	SaveURL(ctx context.Context, code, target string, ttl time.Duration) error
	FindURL(ctx context.Context, code string) (string, error)
	Ping(ctx context.Context) error
	Close() error
}
```

`SaveURL` returns `ErrCodeTaken` on a collision and must honour the TTL; `FindURL` returns `ErrNotFound`
when the code is missing or expired; `Close` must tolerate being called twice. The constructor receives an
`Env` lookup and validates its own variables — core config knows nothing about them. Rate limiting, code
generation and URL validation are not adapter concerns.

### Releases

Every commit that lands on `main` and passes CI is tagged, released, and pushed to Docker Hub by
`.github/workflows/release.yml`. The version comes from the commit subjects since the newest tag:

| Subject | Bump |
| --- | --- |
| `breaking:`, `breaking(api):`, or any type with `!` (`feat!:`) | major |
| `feat:`, `feat(api):` | minor |
| anything else | patch |

Run `.github/next-version.sh` in a full checkout to print the next version without releasing.

Publishing needs two repository settings under **Settings → Secrets and variables → Actions**: a
`DOCKERHUB_USERNAME` variable, and a `DOCKERHUB_TOKEN` secret holding a Docker Hub personal access token
with the **Read & Write** scope. Secrets are never handed to pull requests from forks, and this workflow
only ever triggers on `workflow_run` for CI runs on `main`.

## License

Free for noncommercial use under the [PolyForm Noncommercial License 1.0.0](LICENSE). Commercial use
requires a paid license — <https://zbinski.dev>.
