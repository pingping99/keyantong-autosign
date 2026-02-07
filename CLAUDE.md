# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AbleSci 自动签到脚本 - An automated daily sign-in script for AbleSci.com (科研通), written in Go. Supports multi-account management, dynamic sign-in windows, and long-running daemon mode with Docker support.

Repository: https://github.com/pingping99/keyantong-autosign

## Development Commands

### Build and Run

```bash
# Install dependencies
go mod tidy

# Run directly
go run main.go

# Build executable
go build -o ablesci-sign.exe

# Run built executable
./ablesci-sign.exe
```

### Docker

```bash
# Build image
docker build -t ablesci-sign .

# Run with Docker Compose
docker compose up -d --build

# View logs
docker compose logs -f ablesci-sign

# Stop
docker compose down
```

### Testing

The project does not currently have automated tests. Manual testing is done by running the application and verifying sign-in behavior.

## Configuration

### Multi-Account Mode (Recommended)

Create `data/accounts.json` from the example template:

```bash
cp data/accounts.json.example data/accounts.json
```

Edit with your account credentials. Each account gets independent state tracking in `data/state_<account_hash>.json`.

### Single-Account Mode (Environment Variables)

Set these environment variables when `accounts.json` is absent:
- `ABLESCI_EMAIL` - Account email (required)
- `ABLESCI_PASSWORD` - Account password (required)

### Runtime Configuration

- `CHECK_INTERVAL` - Sign-in check frequency (default: `30m`)
- `DYNAMIC_WINDOW_START` - Daily random window range start (default: `08:00`)
- `DYNAMIC_WINDOW_END` - Daily random window range end (default: `18:00`)
- `DYNAMIC_WINDOW_SPAN` - Window duration (default: `45m`)
- `RETRY_INTERVAL` - Minimum retry interval on failure (default: `10m`)
- `FORCE_SIGN_ON_START` - Force sign-in on startup (default: `true`)
- `TZ` - Timezone (default: `Asia/Shanghai`)
- `DATA_DIR` - Data directory for logs and state (default: `./data`)

## Architecture

The codebase follows a clean, modular architecture with clear separation of concerns:

### Module Structure

```
keyantong/
├── main.go              # Entry point: DI container, startup logic, periodic scheduler
├── config/              # Configuration management
│   └── config.go        # Env var parsing, account loading, AppConfig struct
├── domain/              # Domain models (business entities)
│   ├── account.go       # Account entity (email, password, ID)
│   └── state.go         # SignState entity (last sign date, window info)
├── store/               # State persistence abstraction
│   ├── state_store.go   # StateStore interface (Load/Save)
│   └── file_store.go    # File-based implementation (JSON storage)
├── scheduler/           # Time window utilities
│   └── window.go        # Window validation, random generation, time parsing
├── signer/              # Sign-in orchestration
│   └── signer.go        # Signer interface + AccountSigner implementation
├── service/             # AbleSci API layer
│   └── sign.go          # HTTP calls: login, sign-in, CSRF token extraction
└── client/              # HTTP client abstraction
    └── client.go        # HTTP client with cookie jar management
```

### Key Design Patterns

**Dependency Injection**: `main.go` assembles all components and injects dependencies (follows 100-line principle after refactor from 400+ lines).

**Interface Abstraction**:
- `store.StateStore` - Abstracts state persistence (currently file-based, easily swappable to DB)
- `signer.Signer` - Abstracts sign-in orchestration (supports multiple strategies)

**Separation of Concerns**:
- `config` - Configuration loading only
- `domain` - Pure business entities (no I/O)
- `store` - Persistence layer (no business logic)
- `scheduler` - Time utilities (pure functions)
- `signer` - Business orchestration (coordinates service + store)
- `service` - External API calls only
- `client` - HTTP transport only

### Sign-In Flow

1. **Startup**: `main.go` loads config, initializes store, builds signers
2. **Force Sign-In** (if enabled): All accounts sign in immediately on startup
3. **Periodic Checks**: Every `CHECK_INTERVAL`, each account's signer runs `AttemptSign()`
4. **Dynamic Window Logic** (`signer/signer.go`):
   - Check if current time is within today's dynamic window
   - If no window exists for today, generate one using date-seeded randomization
   - Window persists in state file to ensure consistency across restarts
5. **Throttling**: Skips attempts if within `RETRY_INTERVAL` of last attempt (prevents API spam)
6. **Sign-In Execution** (`service/sign.go`):
   - Fetch CSRF token from login page
   - POST login credentials with CSRF token
   - Cookie jar automatically manages session cookies
   - GET sign-in endpoint with authenticated session
   - If login expired, automatically re-login and retry
7. **State Persistence**: Save result (success/failed/skip) and timestamps to `state_<id>.json`

### Dynamic Sign-In Window Mechanism

To avoid detection of fixed-time automated sign-ins:
- Each day, a random window is generated within `DYNAMIC_WINDOW_START` to `DYNAMIC_WINDOW_END`
- Window duration is `DYNAMIC_WINDOW_SPAN` (e.g., 45 minutes)
- Random seed is based on the date, ensuring same window for the entire day
- Window info is persisted in state file (survives restarts)
- Implemented in `scheduler/window.go:GenerateDynamicWindow()`

### State Management

Each account has independent state stored in `data/state_<account_id>.json`:
- `last_sign_date` - Last successful sign-in date (YYYY-MM-DD)
- `last_attempt_date` - Last attempt date
- `last_attempt_time` - Last attempt time (HH:MM)
- `last_result` - Last result: "success", "failed", or "skip"
- `window_date` - Dynamic window date (YYYY-MM-DD)
- `window_start` - Dynamic window start time (HH:MM)
- `window_end` - Dynamic window end time (HH:MM)

State files are backward compatible - missing fields are auto-generated.

### Logging Strategy

To reduce log noise:
- **Outside window**: Only write to file log (`data/sign.log`), not stdout
- **Within window but throttled**: Only write to file log
- **Sign-in success/failure**: Write to both stdout and file log
- **Errors**: Always write to both stdout and file log

Implemented via dual loggers in `main.go`: standard logger (stdout+file) and `fileLogger` (file only).

## API Integration

The application interacts with AbleSci.com via these endpoints:

1. **GET /site/login** - Fetch CSRF token from login page HTML
2. **POST /site/login** - Submit credentials with CSRF token (form-encoded)
3. **GET /user/sign** - Perform daily sign-in (requires authenticated session)

CSRF token extraction supports multiple HTML patterns (meta tags, hidden inputs). See `interface.md` for full API documentation.

Session cookies (`_identity-frontend`) are managed automatically by `net/http/cookiejar` in `client/client.go`.

## Important Notes

- Account credentials in `data/accounts.json` must never be committed (already in `.gitignore`)
- The application is designed for long-running daemon mode (use with cron or Docker)
- State files prevent duplicate sign-ins on the same day
- Login sessions automatically refresh if expired (retry logic in `signer/signer.go`)
- Docker build uses `go build -o /app/signbot .` to ensure single executable output
- Timezone handling is critical - always use configured `Location` from config for time operations

## Dependencies

- Go 1.21+
- `net/http` - HTTP requests and cookie management
- No external dependencies required (uses only standard library)
