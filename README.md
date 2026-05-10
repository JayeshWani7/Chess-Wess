# ChessWess ♟️

ChessWess is a real-time multiplayer chess application with authentication, lobby matchmaking, and live game sync over WebSockets.

> Vision: evolve standard online chess into a timeline/branching experience (see `PLAN.md`).

## Current Features

- User registration and login (JWT-based auth)
- Create and join games from a lobby
- Configurable time controls (including unlimited)
- Real-time move sync using WebSockets
- Move history persistence in PostgreSQL
- Resign flow and game-over status updates
- Responsive React UI with chessboard, clocks, and move list

## Tech Stack

### Frontend
- React + TypeScript + Vite
- Zustand for state management
- `chess.js` for move logic
- TailwindCSS + Framer Motion

### Backend
- Go (net/http)
- PostgreSQL (game/users/moves persistence)
- Redis (optional cache/realtime support)
- Gorilla WebSocket

### Infra
- Docker + Docker Compose

## Project Structure

```text
.
├── backend/                # Go API + WebSocket server
│   ├── db/                 # DB connection + migrations
│   ├── models/             # Domain models
│   └── server/             # Routes, auth, game + ws handlers
├── frontend/               # React app
│   └── src/
│       ├── components/     # Board and game UI components
│       ├── pages/          # Auth, lobby, and game pages
│       ├── store/          # Zustand stores
│       └── utils/          # API + WebSocket clients
├── docker-compose.yml
├── .env.example
├── setup_db.sql
├── Setup.md
└── PLAN.md
```

## Quick Start (Docker)

### Prerequisites
- Docker + Docker Compose

### Run everything

```bash
docker compose up --build
```

This starts:
- Frontend: `http://localhost:5173`
- Backend: `http://localhost:8080`
- Postgres: `localhost:5432`
- Redis: `localhost:6379`

## Local Development (without full Docker stack)

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

### 4) Run frontend (new terminal)

```bash
cd frontend
npm ci
npm run dev
```

Frontend dev server: `http://localhost:5173`

## Environment Variables

Create `.env` in repository root:

```env
DATABASE_URL=postgres://your_db_user:your_db_password@localhost:5432/your_db_name?sslmode=disable
REDIS_URL=redis://localhost:6379
PORT=8080
JWT_SECRET=REPLACE_WITH_SECURE_RANDOM_SECRET

VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

For local Docker defaults in this repo, you can use:
`postgres://ChessWess:ChessWess@localhost:5432/ChessWess?sslmode=disable`.
For production, use secure DB credentials and enable SSL (e.g., `sslmode=require`).
For JWT secrets, generate a strong random value (e.g., `openssl rand -base64 32`).

## API Overview

### Auth
- `POST /api/auth/register`
- `POST /api/auth/login`

### Games (auth required)
- `GET /api/games` — list open games
- `POST /api/games` — create game
- `GET /api/games/{id}` — get game
- `GET /api/games/{id}/moves` — get move history
- `POST /api/games/{id}/join` — join game
- `POST /api/games/{id}/resign` — resign game

### Realtime
- `GET /ws?game_id=<id>&token=<jwt>`

Common WS messages: `move`, `player_joined`, `player_connected`, `player_disconnected`, `game_over`.

### Health
- `GET /health`

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

## Roadmap

The long-term product direction (timeline branching / multiverse chess) is documented in [`PLAN.md`](./PLAN.md).

## License

No license file is currently included.
