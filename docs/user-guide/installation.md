# Installation

## Prerequisites
Go 1.22+, Node.js 20+ (dashboard), Docker Desktop (optional), Git. Check with `./scripts/check-env.ps1`.

## CLI only
```powershell
go mod tidy
go build -o bin/compatlab.exe ./apps/cli/cmd/compatlab
./bin/compatlab.exe help
```

## Full stack with Docker
```powershell
Copy-Item .env.example .env       # set POSTGRES_PASSWORD and JWT_SECRET
cd apps/web; npm install; cd ../..   # creates package-lock.json (commit it)
docker compose up --build
```
Open `http://127.0.0.1:8081`. PostgreSQL and the API are not published to the host.

## Local development
```powershell
$env:JWT_SECRET = "a-long-random-secret-with-at-least-32-chars"
go run ./apps/api/cmd/server           # in-memory store on :8080
cd apps/web; npm install; npm run dev  # http://127.0.0.1:5173 (proxies /api to :8080)
```
For PostgreSQL set `STORE=postgres` and `DATABASE_URL`; migrations are applied at start-up.
