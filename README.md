# Service Health Dashboard

A local service-health dashboard with a React/TypeScript frontend, a Go health-check API, three sample microservices, and PostgreSQL storage. The API checks each enabled service concurrently with a five-second timeout and records each result in PostgreSQL. The dashboard shows service status and failure details, supports manual refresh, and offers light and dark themes.

## Prerequisites

- Docker Desktop (or another Docker Engine) running
- Node.js and npm installed

## Run locally

Open two terminal windows from the repository root.

### 1. Start the backend stack

```sh
cd app-code/backend
docker compose up --build -d
```

This starts PostgreSQL, the Go health API, and the `user-service`, `order-service`, and `payment-service` containers. PostgreSQL initializes the relational schema and sample service records on API startup. Wait for the containers to start, then check their state with:

```sh
docker compose ps
```

The API endpoint is `http://localhost:8080/api/health`. You can also check the individual sample services at:

- `http://localhost:8081/health` — user-service
- `http://localhost:8082/health` — order-service
- `http://localhost:8083/health` — payment-service

### 2. Start the frontend

In a second terminal, from the repository root:

```sh
cd frontend
npm install
npm run dev
```

Open the local URL printed by Vite, usually [http://localhost:5173](http://localhost:5173). Vite proxies the dashboard's `/api` requests to the health API on port 8080.

## Stop the application

From `app-code/backend`:

```sh
docker compose down
```

The PostgreSQL data volume is retained, so service configuration and health-check history remain available when the stack is started again. Stop the Vite process with `Ctrl+C` in its terminal.

## Run tests and checks

From the repository root, run the backend and frontend checks with:

```sh
(cd app-code/backend && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out && go build ./...)
(cd frontend && npm ci && npm run test:coverage && npm run lint && npm run build)
```

These are also run by the GitHub Actions CI workflow.
The Go coverage profile is written to `app-code/backend/coverage.out`, and the frontend LCOV report is written to `frontend/coverage/lcov.info`. CI uploads both reports as workflow artifacts.

## Troubleshooting

- **Docker daemon is unavailable:** Start Docker Desktop, wait until its engine is ready, then rerun `docker compose up --build -d`.
- **Dashboard reports that the backend is unavailable:** Confirm the Compose containers are running with `docker compose ps`, and open `http://localhost:8080/api/health` directly to inspect the API response.
- **A service shows DOWN:** Check its `/health` endpoint using the URLs above and inspect the backend logs with `docker compose logs health-api user-service order-service payment-service`.
- **PostgreSQL port conflict:** Compose publishes PostgreSQL on host port `5433` and the API connects to PostgreSQL internally through Compose. The dashboard API remains on port `8080`.

See [docs/architecture.md](docs/architecture.md) for the component diagram, request flow, database model, and API details.
