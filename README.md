# shortr

A URL shortener and QR code generator. A Go 1.27 API built on [Fiber v3](https://github.com/gofiber/fiber)
serves the JSON endpoints, the redirects and a plain TypeScript client from the same binary. One Docker
image contains both. Links expire, there are no accounts, and nothing is tracked.

- [Quick start](#quick-start)
- [Configuration](#configuration)
- [API](#api)
- [Client](#client)
- [Development](#development)
- [Production image](#production-image)
- [Deploying](#deploying)
- [Writing an adapter](#writing-an-adapter)
- [Security notes](#security-notes)
- [End-to-end tests](#end-to-end-tests)
- [License](#license)

## Quick start

### docker run (in-memory storage)

Build the production image and run it. With no `URL_PERSISTER` set the API keeps links in memory, so it
needs no other service.

```sh
docker build -f .docker/Dockerfile -t shortr:latest .
docker run --rm -p 8080:8080 shortr:latest
```

Open <http://localhost:8080>. Links live in the process: restarting the container drops them.

If the app is reachable under another origin, set `BASE_URL` so the returned short links point at it:

```sh
docker run --rm -p 8080:8080 -e BASE_URL=https://s.example.com shortr:latest
```

### docker compose (Redis storage)

The development stack runs the API with hot reload, the client watcher and Redis:

```sh
cp .env.example .env
task project:start        # docker volume create shortr-go-cache && docker compose up -d --build --renew-anon-volumes
```

The stack listens on <http://localhost:8080>. Stop it, dropping its volumes, with `task project:stop`.

`.env.example` sets `URL_PERSISTER=redis` and the matching `REDIS_*` variables; change `REDIS_PASSWORD`
before using the stack for anything but local development.

## Configuration

Everything is read from the environment at startup. An invalid value aborts the start with an error naming
the variable.

| Variable | Default | Rules |
| --- | --- | --- |
| `PORT` | `8080` | Integer between 1 and 65535. |
| `BASE_URL` | `http://localhost:8080` | Public origin used to build short links and the client page's Open Graph URLs. Must parse as a URL with an `http`/`https` scheme and a host. Trailing slashes are stripped. |
| `STATIC_DIR` | `./apps/client/dist` | Directory holding the built client. Must exist and be a directory at startup. |
| `URL_TTL` | `30d` | How long a link lives. Go duration syntax (`12h`, `90m`, `30s`) plus an `Nd` day suffix. Must be greater than zero. |
| `URL_MAX_LENGTH` | `4096` | Longest target URL accepted, in bytes (one byte per character for ASCII URLs). Integer between 256 and 65536. The request body cap follows it: `URL_MAX_LENGTH` + 1024 bytes, room for the JSON wrapper. |
| `URL_PERSISTER` | `memory` | Storage adapter: `memory` or `redis`. Unknown names abort the start. |
| `RATE_LIMIT_MODE` | `day` | Rate limit window: `second`, `minute`, `hour` or `day`. |
| `RATE_LIMIT_VALUE` | `30` | Requests allowed per window per client IP. Integer of at least 1. |
| `TRUSTED_PROXIES` | *(empty)* | Comma separated CIDR blocks. When non-empty, `X-Forwarded-For` is trusted and the client IP is taken from the chain; when empty, the header is ignored. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. Logs are JSON on stdout. |

Empty and whitespace-only values fall back to the default for the variables above; a blank required
`REDIS_*` value is an error.

### Adapter: `redis`

Read and validated by the adapter itself, only when `URL_PERSISTER=redis`.

| Variable | Default | Rules |
| --- | --- | --- |
| `REDIS_HOST` | *(required)* | Non-empty host name. |
| `REDIS_PORT` | *(required)* | Integer between 1 and 65535. |
| `REDIS_USER` | *(required)* | Non-empty user name. |
| `REDIS_PASSWORD` | *(required)* | Non-empty. Kept untrimmed and never logged or included in an error. |
| `REDIS_DB` | `0` | Integer of at least 0. |

Keys are stored as `url:<code>` with `SET NX EX`, so expiry is handled by Redis.

## API

Errors share one JSON shape:

```json
{ "error": "invalid_url", "message": "the url must be at most 4096 characters" }
```

The `error` code is stable; the `message` states the reason that actually applied, so a rejected URL says
whether it was too long, not absolute, or carried whitespace.

### `POST /api/shorten`

Requires `Content-Type: application/json`. Rate limited and bot filtered.

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

`201 Created` on success; `expiresAt` is RFC 3339.

| Status | `error` | Cause |
| --- | --- | --- |
| 400 | `invalid_body` | Body is not a JSON object with a `url` field, or `url` is empty. |
| 400 | `invalid_url` | Not an absolute `http`/`https` URL, longer than `URL_MAX_LENGTH`, or carrying whitespace, control characters or user information. The `message` names which of these it was. |
| 403 | `forbidden` | The `User-Agent` matches the blocked crawler list. |
| 413 | `body_too_large` | Body exceeds `URL_MAX_LENGTH` + 1024 bytes (5120 by default). |
| 415 | `unsupported_media_type` | The content type is not JSON. |
| 429 | `rate_limited` | Rate limit exhausted. The limiter sets `Retry-After` on the rejected response; `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` are set on accepted ones. |
| 500 | `internal` | Storage failure, or no free code after 5 attempts. |

### `GET /{code}`

`302 Found` to the target URL with `Cache-Control: no-store`. Neither rate limited nor bot filtered.

`404 not_found` when the code is unknown, expired, or not 12 characters of `[0-9A-Za-z]`.

### `GET /healthz`

`200 {"status":"ok"}` when the adapter answers a ping, `503 unavailable` when it does not. Not bot filtered,
so it works from probes with any user agent.

### Client files

`GET /`, `GET /robots.txt`, `GET /favicon.svg`, `GET /favicon.png`, `GET /apple-touch-icon.png`, `GET /og.png`
and `GET /assets/*` are served from `STATIC_DIR`; assets carry a one hour `max-age`. `GET /` has `BASE_URL`
substituted for `__BASE_URL__`, so the page's `og:url` and `og:image` are absolute. All are bot filtered.

## Client

Plain HTML, CSS and TypeScript, bundled by esbuild into `apps/client/dist` and served by the API. Both
typefaces — Bricolage Grotesque for the interface, IBM Plex Mono for links, hints and micro labels — are
self-hosted: the build emits their woff2 files under `/assets/` next to the bundle, so no external requests are
made at runtime.

**Two modes**, a `role="tablist"` of *Shorten URL* and *QR code*. The chosen mode is stored in `localStorage`
under `shortr.mode` and restored on the next visit; the default is *Shorten URL*.

**Keyboard.** The tabs use a roving tabindex: Left/Right move between them, Home and End jump to the first
and last, and switching clears the previous result. The URL input is focused on load and Enter submits the form.

**Clipboard.** Shorten mode shows the short link as a focusable result button, the host above the 12 character
code; QR mode draws the code as inline SVG modules that animate outward from the centre on a paper tile, with
the short URL beside it as a second copy button. The result is copied automatically as soon as it appears — the
short URL as text, or in QR mode a PNG drawn on a canvas with a four module quiet zone and handed over through
`ClipboardItem`. A toast announces `Copied to clipboard` through an `aria-live` region, and clicking or
activating either button copies again. When the clipboard is unavailable — an insecure context or a denied
permission — the toast reads `Copy failed, click the result to copy manually`.

**Expiry.** Directly under the short link the result card shows when it dies, built from the `expiresAt` of the
shorten response: the nearest whole unit left — days, hours or minutes — picked out in the accent colour, then
the date, for example `Expires in 30 days · 4 Oct 2026`.

**Errors** render in a dedicated `role="alert"` element under the form, the short status code set in mono
before the message. A 429 adds the number of seconds to wait, read from `Retry-After`.

## Development

Everything runs in Docker; no Go or Node toolchain is needed on the host, only Docker and
[Task](https://taskfile.dev).

```sh
task project:start   # api + client watcher + redis, with hot reload
task project:stop    # docker compose down --volumes --remove-orphans
```

The API container runs [air](https://github.com/air-verse/air), rebuilding and restarting the binary on every
Go change; the client container runs the esbuild watcher, writing `apps/client/dist`, which is bind-mounted
read-only into the API at `STATIC_DIR`. `task api:start` brings up the same stack.

| Task | What it does |
| --- | --- |
| `task project:start` | Build and start the development stack. |
| `task project:stop` | Stop it and drop its volumes. |
| `task project:lint` | `api:lint`, `client:lint` and `e2e:lint`. |
| `task project:test` | `api:test` and `client:test`. |
| `task project:e2e` | Playwright suite against the production image. |
| `task api:start` | Start the development stack from the API side. |
| `task api:stop` | Same as `project:stop`. |
| `task api:build` | Build the production image `shortr:latest`. |
| `task api:lint` | golangci-lint over the Go module. |
| `task api:test` | `go test -race -count=1 ./...`. |
| `task client:lint` | Biome check plus `tsc --noEmit`. |
| `task client:test` | Vitest unit tests. |
| `task client:build` | Bundle the client into `apps/client/dist`. |
| `task client:format` | Format the client sources with Biome. |
| `task e2e:lint` | Biome check plus `tsc --noEmit` over `tests/`. |

`task --list-all` prints the full list, including any tasks from a local `Taskfile.local.yml`.

The landing page in `docs/` is maintained by hand and repeats the facts in this README; it is published to
GitHub Pages by `.github/workflows/pages.yml` on every push to `main` that touches `docs/**`, so change the
two together.

## Production image

`.docker/Dockerfile` is a single multi-stage file; its last stage is the production image built by
`task api:build`.

- Base `gcr.io/distroless/static-debian12:nonroot` — no shell, no package manager, no Go or Node toolchain.
- Contents: the static `/shortr` binary (`CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`) and the built
  client in `/srv/client`. Source maps are stripped.
- `ENV STATIC_DIR=/srv/client PORT=8080`, `EXPOSE 8080`, `USER nonroot`, `ENTRYPOINT ["/shortr"]`.

Because the image has no shell, container healthchecks must come from outside; probe `GET /healthz` instead.

## Deploying

Deployment lives in `Taskfile.local.yml`, which is gitignored so every checkout can point at its own registry:

```sh
cp Taskfile.local.example.yml Taskfile.local.yml
# edit the IMAGE variable, for example registry.example.com/shortr:latest
task project:deploy
```

The task builds the production image for `linux/amd64` from `.docker/Dockerfile` and pushes it to `IMAGE`.
The example file ships with `repository.zbinski.dev/shortr:latest`; change it before running the task. Log in
to the target registry first (`docker login`).

## Writing an adapter

An adapter is the only thing that knows how URLs are stored. Nothing else in the codebase learns about a new
backend, so adding one touches `adapters/<name>.go`, its test, and a single case in `adapters.New`.

The contract, from `apps/api/adapters/adapter.go`:

```go
type Env func(key string) (string, bool)

type Adapter interface {
	SaveURL(ctx context.Context, code, target string, ttl time.Duration) error
	FindURL(ctx context.Context, code string) (string, error)
	Ping(ctx context.Context) error
	Close() error
}
```

1. **Implement the four methods.** `SaveURL` returns `adapters.ErrCodeTaken` when the code already exists and
   must honour the TTL. `FindURL` returns `adapters.ErrNotFound` when the code is missing or expired. `Ping`
   backs `/healthz`. `Close` releases the connection and must tolerate being called twice.
2. **Read your own environment.** The constructor receives the `Env` lookup (`os.LookupEnv` in `main`, a map
   in tests) and validates its own variables, naming the offending one in the error. Core config knows nothing
   about them — see how `NewRedis` reads `REDIS_HOST` and friends.
3. **Register it.** Add one `case "<name>":` to `adapters.New`, returning the constructor's result.
4. **Add tests.** A table over missing and invalid environment variables, plus the storage behaviour against a
   fake or in-process server (the Redis adapter uses miniredis).

That is the whole surface. Rate limiting, code generation and URL validation are not adapter concerns.

## Security notes

- **Rate limiting** uses Fiber's `limiter` middleware with a fixed window keyed by client IP, and is attached
  to the `/api` group only — redirects are never limited. The counters live in the process, so with more than
  one replica each process enforces its own limit. Behind a proxy, set `TRUSTED_PROXIES` so the client IP is
  taken from `X-Forwarded-For` rather than the proxy's own address.
- **Not indexable.** Every response carries `X-Robots-Tag: noindex, nofollow, noarchive`, the client page
  carries `<meta name="robots" content="noindex, nofollow">`, and `robots.txt` disallows everything. A request
  whose `User-Agent` contains a known crawler or scraper token gets `403 forbidden` on the client files and the API;
  redirects and `/healthz` stay open so real links and probes keep working.
- **Headers.** `helmet` sets a strict `Content-Security-Policy` (`default-src 'self'`, no inline scripts or
  styles, `frame-ancestors 'none'`, `base-uri 'none'`), plus `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY` and `Referrer-Policy: no-referrer`.
- **Codes** are 12 characters drawn from `[0-9A-Za-z]` with `crypto/rand`, one character at a time to avoid
  modulo bias — roughly 71 bits, so links cannot be enumerated or guessed.
- **Everything expires.** Every link is written with `URL_TTL` (30 days by default) and disappears afterwards:
  Redis expires the key itself, the memory adapter expires on read and sweeps in the background.
- **Input limits.** Target URLs are capped at `URL_MAX_LENGTH` (4096 bytes by default) and bodies at that
  cap plus 1024 bytes for the JSON wrapper, so the longest accepted URL always reaches validation. URLs carrying
  whitespace, control characters or `user:password@` credentials are rejected.

## End-to-end tests

`tests/` holds a Playwright suite that runs against the **production image**, not the development stack:
compose builds the final image, waits for `/healthz`, and runs the browser in the API container's network
namespace so the page is reached over `http://localhost:8080` — a secure context, which the clipboard API
requires.

```sh
task project:e2e
```

## License

shortr is free for noncommercial use under the [PolyForm Noncommercial License 1.0.0](LICENSE); commercial use
requires a paid license — inquiries at <https://zbinski.dev>.
