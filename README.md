# XinuProxyApi

XinuProxyApi is a self-hosted proxy server that exposes OpenAI, Gemini, Claude, Codex, and Grok compatible API endpoints, bundled together with its browser-based Management Center in a single application and release.

Point any OpenAI-, Gemini-, or Claude-compatible client or SDK at your own instance and route requests across multiple CLI accounts with round-robin load balancing — all managed from a built-in web UI.

## Highlights

- OpenAI / Gemini / Claude / Codex / Grok compatible API endpoints.
- OAuth login flows for Codex, Claude Code, and Grok Build.
- Streaming, non-streaming, and WebSocket responses where supported.
- Function calling / tools and multimodal (text + image) input.
- Multiple accounts per provider with round-robin load balancing.
- OpenAI-compatible upstream providers via config (e.g. OpenRouter).
- Built-in SQLite WAL storage for configuration, OAuth credentials, cooldown state, and usage history.
- Bundled single-file Management Center served from the same origin.
- Reusable Go SDK (`sdk/`) for embedding the proxy.

## Repository layout

- `cmd/`, `internal/`, `sdk/`, `test/` — Go proxy server and Management API.
- `web/management-center/` — React + TypeScript + Vite + Bun frontend source.
- `static/management.html` — generated single-file frontend, served at `/` and `/management.html`.
- `scripts/build-management.sh` — reproducible frontend artifact builder.
- `scripts/verify.sh` — frontend and backend verification.
- `config.example.yaml` — annotated configuration template.

The frontend uses hash routing and calls the same-origin Management API under `/v0/management`.

## Install on Linux

### 1. Download a release

Grab the archive that matches your CPU architecture from the repository Releases page:

- `XinuProxyApi_<version>_linux_amd64.tar.gz` — standard x86-64 build (glibc 2.17+ baseline, supports dynamic plugins).
- `XinuProxyApi_<version>_linux_aarch64.tar.gz` — standard arm64 build.
- `*_no-plugin.tar.gz` — portable build for musl-based or older systems (no dynamic plugin support).

```bash
mkdir -p ~/xinuproxyapi && cd ~/xinuproxyapi
tar -xzf XinuProxyApi_<version>_linux_amd64.tar.gz
```

Each archive contains the `xinu-proxy-api` binary, `static/management.html`, `config.example.yaml`, `README.md`, and `LICENSE`.

### 2. Create your config

Copy the template and set a strong management key. The key is hashed on startup and is required for every Management API request.

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml`:

```yaml
# Bind address. Keep 127.0.0.1 when a reverse proxy sits in front.
host: "127.0.0.1"
port: 20128

remote-management:
  # Required. Any non-empty value enables the Management API and web UI login.
  secret-key: "<YOUR_MANAGEMENT_KEY>"
  # Keep the bundled panel pinned to this release.
  disable-auto-update-panel: true
```

Leave `secret-key` empty only if you intentionally want to disable the Management API entirely.

### 3. Run it

```bash
./xinu-proxy-api --config config.yaml
```

The server binds `port` (default `20128`). Verify it locally:

```bash
curl http://127.0.0.1:20128/healthz          # -> 200
curl http://127.0.0.1:20128/v1/models        # -> 401 without a key (expected)
```

Open `http://127.0.0.1:20128/` in a browser to reach the Management Center.

### 4. Run as a systemd service (optional)

Create `/etc/systemd/system/xinuproxyapi.service`:

```ini
[Unit]
Description=XinuProxyApi
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/xinuproxyapi
ExecStart=/opt/xinuproxyapi/xinu-proxy-api --config /opt/xinuproxyapi/config.yaml
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now xinuproxyapi.service
sudo systemctl status xinuproxyapi.service
```

### 5. Expose it (optional)

To serve over HTTPS, put a reverse proxy (Nginx, Caddy, Cloudflare Tunnel, etc.) in front of `127.0.0.1:20128` and terminate TLS there. Because the app is same-origin, no extra frontend configuration is required — the web UI automatically talks to the backend on whatever host it is served from.

When exposing the Management API beyond localhost, also set `remote-management.allow-remote: true` and use a long, random `secret-key`.

### 6. Run with Docker (optional)

You can also build and run XinuProxyApi in a container using Docker Compose:

```bash
cp config.example.yaml config.yaml
# Interactive helper (pre-built image or local source build)
./docker-build.sh

# Or build and start directly with Docker Compose
XINU_PROXY_IMAGE=xinu-proxy-api:local docker compose up -d --build
```

For cluster / control-plane worker mode, see `docker-compose.cluster.yml` and `.env.cluster.example`.

## Build from source

Requirements: Go 1.26+, Bun 1.3.14 (Node.js 24 for frontend CI parity).

```bash
# Full build (frontend panel + Go binary)
make build VERSION=dev

# Equivalent explicit steps
VERSION=dev ./scripts/build-management.sh   # writes static/management.html
go build -o xinu-proxy-api ./cmd/server
```

Frontend development server:

```bash
cd web/management-center
bun install --frozen-lockfile
bun run dev
```

Verify everything (frontend tests, lint, build, Go tests, Go compile):

```bash
./scripts/verify.sh
```

## The Management Center (Web UI)

The bundled web UI is a single-file React app served at the site root `/` (browsers get the UI, API clients get JSON) and also at `/management.html`. It is English-only and ships with a two-mode editorial theme — a warm ink-on-charcoal dark default and a light paper mode — with an animated scanline transition between them.

### Logging in

Open the site in a browser and enter your **management key** (the `secret-key` from `config.yaml`). The connection URL is detected automatically from the address you are visiting, so there is nothing else to configure. Enable "Remember password" to persist the key in local browser storage.

### Sections

- **Dashboard** — live request activity, connection status, and a health summary at a glance.
- **Usage** — persistent hourly token and estimated cost breakdown per provider and model.
- **AI Providers** — unified card grid for Custom Providers, OAuth Providers, Free Tier (OpenCode Free), and API Key Providers.
- **Models** — per-provider model catalog discovery, free-only filtering, and global/per-account model toggles.
- **Auth Files** — inspect and manage per-account credential files created by OAuth login flows.
- **Quota** — per-account quota tracking cards across providers.
- **Config** — structured visual editor and full YAML editor for the server configuration.
- **Console Log** — real-time SSE server console stream with live log-level switching (`ERR`, `WRN`, `INF`, `DBG`).

### Security notes

- The Management Center shell is publicly reachable, but every Management API operation requires the management key.
- Treat the management key as highly privileged: the Management API can read and change configuration, access stored credentials, and drive outbound API calls. Use a long random value and rotate it if exposed.
- Never commit `config.yaml`, `.env`, management keys, provider credentials, auth files, or logs.

## Release behavior

Release archives contain the platform binary, `static/management.html`, `config.example.yaml`, documentation, and the license. The bundled panel is available without a first-start download. Set `disable-auto-update-panel: true` to keep the UI pinned to the shipped release.

## License

The backend and Management Center are MIT licensed. The Management Center license is preserved at `web/management-center/LICENSE`.
