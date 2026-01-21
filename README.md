# juango

> Like Django, but made by Juan in Go.

A scaffolding CLI and reusable libraries for building Go + Vite/React web applications with batteries included: OIDC authentication, session management, admin mode, audit logging, and more.

## Disclaimer

This project has been created with [Claude](https://claude.ai) (Anthropic's AI assistant). I had several internal projects that shared the same patterns - OIDC auth, session management, admin mode, etc. - but the code was just copy-pasted between them with no actual shared library. I used Claude to extract the minimum common denominator from these projects and refactor it into this reusable framework. The patterns come from real-world production code, but the extraction, refactoring, and documentation were done through AI pair programming.

## Features

- **CLI scaffolding** - `juango init myapp` creates a complete project structure
- **OIDC authentication** - Plug-and-play with any OIDC provider (Entra ID, Okta, Auth0, etc.)
- **Session management** - Secure cookie-based sessions with SQLite backend
- **Admin mode** - Time-limited elevated privileges with audit logging
- **User impersonation** - Debug issues as another user (admin only)
- **Audit logging** - Track all sensitive operations with actor/impersonator awareness
- **Frontend serving** - Dev proxy to Vite, embedded SPA in production
- **SQLite with WAL** - Simple, fast, single-file database
- **Background tasks** - Asynq integration for Redis-backed job queues
- **React UI library** - Pre-built components with shadcn/ui styling

## Installation

```bash
go install github.com/juanfont/juango@latest
```

## Quick Start

```bash
# Create a new project
juango init myapp
cd myapp

# Configure OIDC (edit config.yml with your provider details)
cp config.example.yml config.yml
vim config.yml

# Start development (runs Vite + Go concurrently)
juango dev

# Visit http://localhost:8080
```

## Project Structure

A generated project looks like this:

```
myapp/
├── cmd/myapp/
│   ├── myapp.go           # Entry point
│   └── cli/
│       ├── root.go        # CLI setup
│       └── serve.go       # Server command
├── internal/
│   ├── api/
│   │   └── app.go         # API routes and handlers
│   ├── database/
│   │   ├── db.go          # Database wrapper
│   │   └── sql/
│   │       └── schema.sql # Your tables
│   └── types/
│       └── config.go      # Configuration struct
├── frontend/
│   ├── src/
│   │   ├── App.tsx        # React app
│   │   ├── pages/         # Your pages
│   │   └── components/    # Your components
│   ├── package.json
│   └── vite.config.ts
├── app.go                 # Main application
├── go.mod
├── config.example.yml
└── .gitignore
```

## CLI Commands

### `juango init <project-name>`

Creates a new project with all the scaffolding.

```bash
juango init myapp                           # Uses github.com/user/myapp
juango init myapp -m github.com/org/myapp   # Custom module path
```

### `juango dev`

Starts the development environment:
- Vite dev server with HMR on port 5173
- Go server on port 8080 (proxies to Vite for frontend)

```bash
cd myapp
juango dev
```

### `juango version`

Shows version information.

## Go Library

The `github.com/juanfont/juango` module provides reusable packages:

### `juango/frontend`

SPA serving with automatic dev/prod detection.

```go
import "github.com/juanfont/juango/frontend"

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
    router := mux.NewRouter()
    frontend.Setup(router, frontendFS, "frontend/dist")
}
```

### `juango/auth`

OIDC authentication and session middleware.

```go
import "github.com/juanfont/juango/auth"

// Create OIDC provider
provider, _ := auth.NewOIDCProvider(ctx, auth.OIDCProviderConfig{
    ServerURL: "http://localhost:8080",
    OIDCConfig: types.OIDCConfig{
        Issuer:       "https://login.microsoftonline.com/...",
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
    },
})

// Setup handlers
handlers := auth.NewOIDCHandlers(provider, sessionStore, "session", userStore, auditLogger)
router.HandleFunc("/api/auth/login", handlers.LoginHandler)
router.HandleFunc("/api/auth/logout", handlers.LogoutHandler).Methods("POST")
router.HandleFunc(provider.CallbackPath(), handlers.CallbackHandler)

// Protect routes
middleware := auth.NewSessionMiddleware(sessionStore, "session", userStore, auditLogger, 30*time.Minute)
router.HandleFunc("/api/protected", middleware.RequireAuth(myHandler))
router.HandleFunc("/api/admin-only", middleware.RequireAuth(middleware.RequireAdmin(adminHandler)))
```

### `juango/admin`

Admin mode and impersonation handlers.

```go
import "github.com/juanfont/juango/admin"

handlers := admin.NewHandlers(sessionStore, "session", userStore, auditLogger, 30*time.Minute)

// Admin mode (time-limited elevation)
router.HandleFunc("/api/admin/mode/enable", middleware.RequireAdmin(handlers.AdminModeEnableHandler)).Methods("POST")
router.HandleFunc("/api/admin/mode/disable", middleware.RequireAdmin(handlers.AdminModeDisableHandler)).Methods("POST")
router.HandleFunc("/api/admin/mode/status", handlers.AdminModeStatusHandler).Methods("GET")

// Impersonation
router.HandleFunc("/api/admin/impersonate/start", middleware.RequireAdminMode(handlers.ImpersonationStartHandler)).Methods("POST")
router.HandleFunc("/api/admin/impersonate/stop", handlers.ImpersonationStopHandler).Methods("POST")
```

### `juango/middleware`

Common HTTP middleware.

```go
import "github.com/juanfont/juango/middleware"

router.Use(middleware.Logging(logger))    // Request logging with zerolog
router.Use(middleware.Metrics())          // Prometheus metrics
router.Use(middleware.Recovery())         // Panic recovery
router.Use(middleware.CORS(nil))          // CORS (nil = permissive defaults)
```

### `juango/database`

SQLite helpers with WAL mode and migrations.

```go
import "github.com/juanfont/juango/database"

//go:embed sql/schema.sql
var schema string

db, _ := database.New("app.db", schema)
defer db.Close()

// Transactions
db.WithTx(ctx, func(tx *sqlx.Tx) error {
    // ...
    return nil
})
```

### `juango/tasks`

Asynq task queue wrappers.

```go
import "github.com/juanfont/juango/tasks"

// Client (enqueue tasks)
client := tasks.NewClient("localhost:6379")
client.Enqueue(ctx, "email:send", payload)

// Server (process tasks)
server := tasks.NewServer("localhost:6379", 10)
server.Register("email:send", handleSendEmail)
server.Start()
```

### `juango/types`

Common types used across packages.

```go
import "github.com/juanfont/juango/types"

// types.User, types.OIDCClaims, types.AuditLog,
// types.AdminModeState, types.ImpersonationState, etc.
```

## Frontend Components

Scaffolded projects include a complete React frontend with:

- **Contexts**: `AuthContext`, `AdminContext`, `BreadcrumbContext`
- **Components**: `ProtectedRoute`, `Layout`, UI components (Button, Card, etc.)
- **Utilities**: `ApiClient` for backend communication, `cn()` for class names

### Example App.tsx

```tsx
import { Routes, Route } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { AdminProvider } from './contexts/AdminContext'
import { ProtectedRoute } from './components/ProtectedRoute'
import Layout from './components/Layout'

export default function App() {
  return (
    <AuthProvider>
      <AdminProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/" element={<ProtectedRoute><Layout /></ProtectedRoute>}>
            <Route index element={<Home />} />
          </Route>
        </Routes>
      </AdminProvider>
    </AuthProvider>
  )
}
```

## Configuration

Example `config.yml`:

```yaml
listen_addr: ":8080"
advertise_url: "http://localhost:8080"
admin_mode_timeout: 30m

database:
  path: "myapp.db"

session:
  cookie_name: "myapp_session"
  cookie_expiry: 24h
  authentication_key: "32-byte-key-for-authentication!!"  # exactly 32 bytes
  encryption_key: "32-byte-key-for-encryption-here"      # exactly 32 bytes

oidc:
  issuer: "https://login.microsoftonline.com/{tenant}/v2.0"
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  scopes:
    - openid
    - profile
    - email

redis:
  addr: "localhost:6379"

logging:
  level: info
  format: text
```

Generate secure keys:

```bash
openssl rand -hex 16  # generates 32-character hex string
```

## Testing

The project includes comprehensive integration tests:

```bash
# Run all tests (skips browser tests if Chrome not available)
go test ./integration/... -v

# Run with short flag (skips integration tests)
go test ./... -short

# Run in CI with browser tests
sudo apt-get install -y chromium-browser
go test ./integration/... -v
```

### Test Coverage

- **API Tests**: Session management, OIDC flow, protected endpoints, logout
- **Browser Tests**: Page loading, login flow with real Chrome (headless)

## Building for Production

```bash
# Build frontend
cd frontend && npm run build && cd ..

# Build Go binary (embeds frontend)
go build -o myapp ./cmd/myapp

# Run
./myapp serve -c config.yml
```

Or use GoReleaser:

```bash
goreleaser release --snapshot --clean
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Browser                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Go HTTP Server                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Middleware │  │    Auth     │  │      Frontend       │  │
│  │  - Logging  │  │  - OIDC     │  │  - Dev: Vite proxy  │  │
│  │  - Metrics  │  │  - Session  │  │  - Prod: Embedded   │  │
│  │  - Recovery │  │  - Admin    │  │                     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│                              │                               │
│                              ▼                               │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                     Your API Handlers                    ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │  SQLite  │   │  Redis   │   │   OIDC   │
        │   (WAL)  │   │ (tasks)  │   │ Provider │
        └──────────┘   └──────────┘   └──────────┘
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

---

Built with Claude and caffeine.
