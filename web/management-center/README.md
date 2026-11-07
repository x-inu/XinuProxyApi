# XinuGate Management Center

A single-file Web UI (React + TypeScript) for operating and monitoring **XinuGate** via its **Management API** (`/v0/management`).

**Main Repository**: https://github.com/x-inu/XinuGate

The Web UI ships bundled with `xinugate` (`static/management.html`) and is served at both `/` (for browsers) and `/management.html` on the gateway port (`20128` by default).

## Quick start

### Option A: Use the bundled Web UI (recommended)

1. Start your `xinugate` service.
2. Open `http://<host>:20128/` or `http://<host>:20128/management.html`.
3. Enter your **management key** (`remote-management.secret-key`) to sign in.

### Option B: Run the development server

```bash
bun install --frozen-lockfile
bun run dev
```

Open `http://localhost:5173` in a browser.

### Option C: Build a single HTML file

```bash
bun install --frozen-lockfile
bun run build
```

- Output: `dist/index.html` (all assets inlined into a single file).
- From the repository root, `./scripts/build-management.sh` builds and copies the artifact to `static/management.html`.

## Features

- **Dashboard**: live request activity, connection health summary, and active model overview.
- **Usage**: persistent hourly token and estimated cost ledger backed by SQLite or JSON.
- **AI Providers**: unified card grid for Custom Providers (OpenAI-compatible), OAuth Providers (Antigravity, Codex, Claude Code, Kimi, Meta AI, xAI Grok, Devin, Vertex), Free Tier (OpenCode Free), and API Key Providers.
- **Models**: per-provider model discovery, free-only filtering, and global/per-account model availability toggles.
- **Auth Files**: upload, inspect, toggle, and manage OAuth credential files, excluded models, and aliases.
- **Quota Management**: per-account quota tracking across providers.
- **Config**: visual configuration editor and full YAML editor with diff preview.
- **Console Log**: real-time SSE server console log stream with live level switching (`ERR`, `WRN`, `INF`, `DBG`), search, copy, and export.
- **About**: build/version metadata and local session storage management.

## Tech Stack

- React 19 + TypeScript 6.0
- Vite 8 (`vite-plugin-singlefile`)
- Zustand
- Axios
- react-router-dom v7 (`HashRouter`)
- Motion
- CodeMirror 6
- SCSS Modules
- i18next (English-only catalog)

## Development Scripts

```bash
bun run dev        # Vite dev server
bun run build      # tsc + Vite single-file build
bun run preview    # preview dist locally
bun run test       # Bun unit test suite
bun run lint       # ESLint
bun run verify     # test + lint + build
bun run format     # Prettier
bun run type-check # tsc --noEmit
```

## License

MIT
