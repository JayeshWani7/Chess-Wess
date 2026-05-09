# Start Postgres + Redis
docker compose up postgres redis -d

# Backend
cd "Chess Wess/backend"
go run main.go

# Frontend (new terminal)
cd "Chess Wess/frontend"
npm run dev
