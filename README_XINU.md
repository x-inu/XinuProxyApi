# XinuProxyApi

XinuProxyApi combines a multi-provider proxy backend and its browser Management Center in one repository and release pipeline.

## Layout

- `cmd/`, `internal/`, `sdk/`, `test/`: Go proxy server and Management API.
- `web/management-center/`: React, TypeScript, Vite, and Bun frontend source.
- `static/management.html`: generated single-file frontend served at `/management.html`.
- `scripts/build-management.sh`: reproducible frontend artifact builder.
- `scripts/verify.sh`: frontend and backend verification.

The frontend keeps hash routing and calls the same-origin Management API under `/v0/management`.

## Requirements

- Go 1.26 or newer.
- Bun 1.3.14.
- Node.js 24 for parity with frontend CI.

## Build

Build the complete application:

```bash
make build VERSION=dev
```

Equivalent commands:

```bash
VERSION=dev ./scripts/build-management.sh
go build -o xinu-proxy-api ./cmd/server
```

The server resolves the generated panel from `static/management.html` when `config.yaml` is in the project directory. Set `MANAGEMENT_STATIC_PATH` when the runtime config or static file lives elsewhere.

## Development

Run the Management Center development server:

```bash
cd web/management-center
bun install --frozen-lockfile
bun run dev
```

Build and run the Go server:

```bash
go run ./cmd/server
```

## Verification

```bash
./scripts/verify.sh
```

This runs the frontend tests, lint, production build, all Go tests, and a final Go compile.

## Docker

```bash
docker compose build
```

The multi-stage Dockerfile builds the Management Center with Bun, builds the Go server, and copies `management.html` into the final image. No frontend toolchain is installed in the runtime image.

## Release behavior

Official release archives should contain:

- the platform binary;
- `static/management.html`;
- configuration, documentation, and license files.

The bundled panel is available without a first-start download. Runtime panel updates remain compatible with the existing `remote-management` settings. Set `disable-auto-update-panel: true` for a panel pinned to the application release.

## Security

- Never commit `config.yaml`, `.env`, management keys, provider credentials, auth files, or logs.
- The Management Center shell at `/management.html` is public, but Management API operations require the management key.
- Treat the management key as highly privileged because the Management API can access configuration, credentials, and outbound API-call functionality.

## License

The backend and Management Center are MIT licensed. The Management Center license is preserved at `web/management-center/LICENSE`.
