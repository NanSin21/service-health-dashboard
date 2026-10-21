# Service health backend

## Start the backend and its microservices

This Compose stack runs PostgreSQL, the health API, and each sample microservice in its own container. Run from this directory:

```sh
docker compose up --build -d
```

The dashboard API is available at `http://localhost:8080/api/health`. The three services expose their health endpoints on the host at:

- `http://localhost:8081/health` — user-service
- `http://localhost:8082/health` — order-service
- `http://localhost:8083/health` — payment-service

Inside Compose, the health API checks `http://user-service:8081/health`, `http://order-service:8082/health`, and `http://payment-service:8083/health` by container DNS name. On startup, it creates the relational tables and inserts the sample services if missing. `health_checks.service_id` references `services.id`, and check history is retained in PostgreSQL. The database volume persists across restarts.

PostgreSQL is published on host port `5433` to avoid conflicts with other local PostgreSQL containers; the Compose health API connects to it by the `postgres` service name.

Set `DATABASE_URL` to override the database connection. To stop the stack:

```sh
docker compose down
```

Each API request checks enabled services from PostgreSQL and stores each result. Response times are retained in the database; the dashboard shows service, status, and details.
