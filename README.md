# ChessWess ♟️

ChessWess is a real-time multiplayer chess application built around a core mechanic: **timeline branching**. Players can rewind moves and split the game into parallel timelines, spending an energy resource to reshape the past.

> Full product direction is documented in [`PLAN.md`](./PLAN.md).

---

## Features

### Authentication
- User registration and login with JWT-based auth
- Protected routes and middleware for all game endpoints

### Lobby & Matchmaking
- Create games with configurable time controls (including unlimited)
- Color selection (white/black) when creating a game
- Browse and join open games from a lobby
- Game history page with paginated results and win/loss/draw filters

### Core Chess
- Real-time move sync over WebSockets with atomic DB transactions
- Server-side legal move validation via `notnil/chess`
- Checkmate and stalemate auto-detection with automatic game-over broadcast
- Pawn promotion support
- Resign flow
- Click-to-move and drag-and-drop board interaction

### Timeline Branching (Multiverse Chess)
- Every move is stored as a node in an immutable DAG (directed acyclic graph)
- **Rewind & Branch** — rewind to any past position to create a new parallel timeline
- **Timeline switching** — jump between timelines mid-game (costs energy)
- **Timeline renaming** — give custom names to each branch
- **Multiple timelines per game** — all timelines visible and navigable in a graph view
- Dynamic FEN snapshotting for efficient board-state reconstruction at any node
- Node windowing — paginated loading for large timelines (`node_limit` query param)
- Replay endpoint — reconstruct root-to-node path via recursive CTE + snapshot hydration
- Real-time WebSocket events: `timeline_created`, `timeline_renamed`, `timeline_switched`

### Energy System
- Each player starts with **15 energy** per game
- Energy is spent on timeline operations:
  - Rewinding turns (cost scales with number of turns rewound)
  - Switching timelines
  - Locking a timeline (3 energy)
- Full transaction log of energy spend and refunds
- Timeline **locking** — spend energy to prevent an opponent from altering a favorable timeline
- Timeline **collapse** — defensive mechanic when paradox count exceeds threshold
- Energy panels displayed in-game for both player and opponent (opponent energy is visible)
- Toast notifications when energy is insufficient

### Bot Opponents
Seven difficulty tiers, all playable from the lobby:

| Rating | Behavior |
|--------|----------|
| 400    | Random moves |
| 600    | Mostly random, occasionally captures |
| 800    | Always captures if possible |
| 1000   | 1-ply material evaluation |
| 1200   | 1-ply positional evaluation |
| 1400   | Minimax depth 2 with alpha-beta pruning |
| 1600   | Minimax depth 3 with alpha-beta pruning |

Bots at 1000+ rating also probabilistically spend energy to lock timelines when ahead.

### Game History & Review
- Paginated game history with outcome filters (all / win / loss / draw)
- Game review page — step through every move with ← → arrow keys or click-to-navigate
- Board flips based on your color in the reviewed game
- Shareable review links (copy-to-clipboard)
- Fallback to `game_moves` table if timeline data is unavailable

### Timeline Graph Visualization
- ReactFlow DAG visualization of all timelines with Dagre auto-layout
- Minimap, zoom, and pan controls
- Node inspector — click any node to see FEN, move, and turn number
- Timeline stats panel — material count, piece counts, material advantage
- Load more / load full controls for large graphs

### Observability (backend package — ready, pending server wiring)
- Prometheus metrics registry with 7 instruments:
  - `chesswess_move_duration_seconds` (histogram)
  - `chesswess_move_validation_errors_total` (counter)
  - `chesswess_db_query_duration_seconds` (histogram)
  - `chesswess_ws_connections_active` (gauge)
  - `chesswess_ws_disconnects_total` (counter)
  - `chesswess_http_requests_total` (counter)
  - `chesswess_http_request_duration_seconds` (histogram)
- Structured JSON logger (`slog`) with `GameFields` helper for game/timeline/node context
- HTTP instrumentation middleware (response writer wrapper with status capture)

---

## Tech Stack

### Frontend
- React + TypeScript + Vite
- Zustand for state management (LRU-based timeline memory budget)
- `chess.js` for move logic
- ReactFlow + Dagre for timeline graph
- TailwindCSS + Framer Motion

### Backend
- Go (`net/http`)
- PostgreSQL — games, users, nodes, timelines, energy, timeline metadata
- Redis (optional cache/realtime support)
- Gorilla WebSocket
- Prometheus client (observability package)

### Infra
- Docker + Docker Compose

---

## Project Structure

```text
.
├── backend/
│   ├── db/                 # DB connection, migrations, query helpers
│   ├── models/             # Domain models (game, node, energy, user)
│   ├── observability/      # Prometheus metrics, structured logger, HTTP middleware
│   └── server/             # Routes, auth, game handlers, WebSocket, bot engine, energy, timeline
├── frontend/
│   └── src/
│       ├── components/     # Board, energy panels, timeline graph, game UI
│       ├── hooks/          # useEnergy
│       ├── pages/          # Auth, lobby, game, history, review
│       ├── store/          # Zustand stores (game, auth, timeline memory)
│       └── utils/          # API client, WebSocket client, energy math
├── docker-compose.yml
├── .env.example
└── PLAN.md
```

---

## Quick Start (Docker)

### Prerequisites
- Docker + Docker Compose

```bash
docker compose up --build
```

This starts:
- Frontend: `http://localhost:5173`
- Backend: `http://localhost:8080`
- Postgres: `localhost:5432`
- Redis: `localhost:6379`

---

## Local Development

### 1) Configure environment

```bash
cp .env.example .env
```

### 2) Start Postgres + Redis

```bash
docker compose up postgres redis -d
```

### 3) Run backend

```bash
cd backend
go run main.go
```

### 4) Run frontend

```bash
cd frontend
npm ci
npm run dev
```

Frontend dev server: `http://localhost:5173`

---

## Environment Variables

```env
DATABASE_URL=postgres://your_db_user:your_db_password@localhost:5432/your_db_name?sslmode=require
REDIS_URL=redis://localhost:6379
PORT=8080
JWT_SECRET=REPLACE_WITH_SECURE_RANDOM_SECRET

VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

For local Docker development, set `sslmode=disable` and use the credentials from `docker-compose.yml`. **Do not use these in production.**

---

## API Reference

### Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/auth/register` | Register a new user |
| POST | `/api/auth/login` | Login, returns JWT |

### Games _(auth required)_
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/games` | List open (pending) games |
| POST | `/api/games` | Create a game |
| GET | `/api/games/{id}` | Get game details |
| GET | `/api/games/{id}/moves` | Get move history |
| POST | `/api/games/{id}/join` | Join a game |
| POST | `/api/games/{id}/resign` | Resign |
| GET | `/api/games/{id}/timeline` | Get all timelines + nodes (`?node_limit=N`) |
| POST | `/api/games/{id}/timeline` | Rename a timeline |
| GET | `/api/games/{id}/replay` | Reconstruct root-to-node path (`?node_id=...`) |
| POST | `/api/games/bot` | Create a bot game (`bot_rating`: 400–1600) |
| GET | `/api/games/history` | Paginated game history (`?page=&limit=&filter=`) |

### Energy _(auth required)_
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/games/{id}/energy` | Get your energy |
| GET | `/api/games/{id}/energy/{player_id}` | Get opponent's energy |
| POST | `/api/games/{id}/energy/spend` | Spend energy |
| POST | `/api/games/{id}/energy/refund` | Refund energy |
| POST | `/api/games/{id}/energy/lock-timeline` | Lock a timeline (costs 3 energy) |
| GET | `/api/games/{id}/energy/timeline-status` | Get timeline lock/metadata |

### Nodes _(auth required)_
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/nodes/{id}/branches` | Get child branches of a node |

### Users _(auth required)_
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users/{id}` | Get user info |

### Realtime
```
GET /ws?game_id=<id>&token=<jwt>
```

**Inbound messages (client → server):**
- `move` — `{ uci, timeline_id?, parent_node_id? }`
- `rewind` — `{ node_id }`
- `switch_timeline` — `{ timeline_id }`
- `ping`

**Outbound messages (server → client):**
- `move`, `game_over`, `player_joined`, `player_connected`, `player_disconnected`
- `timeline_created`, `timeline_renamed`, `timeline_switched`
- `pong`, `error`

### Health
```
GET /health  →  {"status":"ok"}
```

---

## Development Commands

### Backend

```bash
cd backend
go test ./...
go build ./...
```

### Frontend

```bash
cd frontend
npm ci
npm run build
```

---

## License

This project is currently unlicensed (no LICENSE file is present yet).
