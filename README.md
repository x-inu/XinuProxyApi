# XinuGate

**XinuGate** is a self-hosted, all-in-one AI Gateway. It lets you connect your OAuth accounts (Claude Code, OpenAI Codex, Google Antigravity, Grok, Kimi, Meta AI, Devin) and API keys (OpenAI, Gemini, OpenRouter, Nous, etc.) and use them all through **one standard endpoint** (`http://localhost:20128/v1`).

Everything — the proxy server, the SQLite database, and the Web Management Center — ships inside **a single `xinugate` binary**. No external database or web server is required.

---

## Quick Install (1 Command — Recommended)

On any Linux server or VPS (x86_64 or ARM64), run:

```bash
curl -fsSL https://raw.githubusercontent.com/x-inu/XinuGate/main/install.sh | sudo bash
```

**What this command does automatically:**
1. Downloads the latest `xinugate` binary for your CPU and verifies its SHA-256 checksum.
2. Creates a ready-to-use `config.yaml` with a freshly generated **Web UI Login Key** and **Client API Key**.
3. Enables and starts `xinugate.service` in the background (with SQLite storage enabled).
4. Prints your login URL and keys right in the terminal.

---

## How to Use XinuGate in 3 Steps

### Step 1 — Open the Web UI
Open **`http://<your-server-ip>:20128/`** in your browser and sign in using the **Web UI Login Key** printed by the installer (or the `secret-key` from your `config.yaml`).

### Step 2 — Connect Your AI Providers
Click **AI Providers** in the left sidebar:
- **OAuth Providers**: Click any provider card (e.g., *Antigravity*, *Claude Code*, *OpenAI Codex*, *xAI Grok*, *Kimi*) and click **Login with OAuth** to sign in with your account.
- **Custom / API Key Providers**: Click **+ Add Custom Provider** or select *Claude / Gemini / OpenAI* to add your API keys and base URLs.
- **Models**: Open the **Models** menu to fetch available models, filter free-tier models (**Free only**), and toggle specific models on or off.

### Step 3 — Point Your AI Apps to XinuGate
In Cursor, OpenCode, Cline, Roo Code, or any OpenAI/Anthropic SDK, set:
- **Base URL**: `http://<your-server-ip>:20128/v1`
- **API Key**: Your **Client API Key** (manageable anytime under **Config → API Keys**).

Test it from your terminal:
```bash
curl http://127.0.0.1:20128/v1/models \
  -H "Authorization: Bearer <YOUR_CLIENT_API_KEY>"
```

---

## Alternative Installation Methods

### Option A: Run with Docker Compose

```bash
git clone https://github.com/x-inu/XinuGate.git
cd XinuGate
cp config.example.yaml config.yaml
# Edit config.yaml and set remote-management.secret-key
XINUGATE_IMAGE=xinugate:local docker compose up -d --build
```

### Option B: Manual Binary Download (Without Installer)

1. Download the archive for your architecture from the [Releases page](https://github.com/x-inu/XinuGate/releases):
   - `XinuGate_<version>_linux_amd64.tar.gz` — Linux x86_64 (glibc)
   - `XinuGate_<version>_linux_amd64_no-plugin.tar.gz` — Linux x86_64 static (Alpine / musl)
   - `XinuGate_<version>_linux_aarch64_no-plugin.tar.gz` — Linux ARM64 static
2. Extract and configure:
   ```bash
   mkdir -p ~/xinugate && cd ~/xinugate
   tar -xzf XinuGate_<version>_linux_amd64.tar.gz
   cp config.example.yaml config.yaml
   ```
3. Set `remote-management.secret-key` in `config.yaml`, then run:
   ```bash
   ./xinugate --config config.yaml -sqlitestore -sqlitestore-path data.sqlite
   ```

---

## Web UI Overview

- **Dashboard** — Real-time request throughput, success rate, and active provider health.
- **Usage** — Persistent SQLite token & estimated USD cost ledger per provider and model.
- **AI Providers** — Unified card grid for Custom Providers, OAuth Accounts, Free Tier pools, and API Keys.
- **Models** — Live model catalog discovery, universal free-model detection, and per-model toggles.
- **Auth Files** — Manage OAuth credential files, model exclusions, and model aliases.
- **Quota** — Per-account quota bars and reset countdown timeline.
- **Config** — Visual settings editor and YAML editor with live diff preview.
- **Console Log** — Real-time SSE server terminal output with live log-level switching (`ERR`, `WRN`, `INF`, `DBG`).

---

## Build from Source

Requirements: **Go 1.26+** and **Bun 1.3.14**.

```bash
# Build embedded Web UI + Go binary (outputs ./xinugate)
make build VERSION=v0.1.0

# Run full test & verification suite (Frontend + Go)
./scripts/verify.sh
```

## License

MIT License — see [LICENSE](LICENSE).
