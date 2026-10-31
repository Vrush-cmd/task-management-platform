# TaskFlow

A small task-management platform built in Go with a public API gateway and a
separate task service. The gateway forwards REST requests to the task service,
which owns task business logic and PostgreSQL persistence. There are no
frameworks: the services use `net/http`, `database/sql`, and plain SQL.

## Requirements

- Go 1.22 or newer (the router uses 1.22's method and path-parameter patterns)
- Docker with Docker Compose (for PostgreSQL)

## Run it locally

```bash
# 0. First time only: download dependencies and create go.sum.
go mod tidy

# 1. Start PostgreSQL. The tasks table is created automatically on first start.
docker compose up -d db

# 2. Configure the app (values in .env.example match docker-compose.yml).
cp .env.example .env
set -a; source .env; set +a

# 3. Start the task service locally on http://localhost:8081
go run ./cmd/task-service
```

In a second terminal, start the gateway with:

```bash
TASK_SERVICE_URL=http://localhost:8081 go run ./cmd/api
```

The public endpoint is then `http://localhost:8080`.

For the complete microservices-style deployment, run the gateway and task
service in containers:

```bash
docker compose up --build
```

The public API is available at `http://localhost:8080` by default; the task
service is private to the Compose network on port 8081. If port 8080 is already
in use, choose another host port:

```bash
GATEWAY_PORT=8090 docker compose up --build
```

The gateway still listens on port 8080 inside the Compose network; only the
host-facing port changes.

The React frontend is available at `http://localhost:3000` when the full
Compose stack is running. For frontend-only development:

```bash
cd frontend
npm install
npm run dev
```

Vite proxies `/api` requests to the gateway at `http://localhost:8080`, so
start the backend stack first. The production frontend container uses Nginx
to serve the static React build and proxy `/api` to the gateway service.

Tasks live in the `pgdata` Docker volume, so they survive restarts of both the
API and the database container. `docker compose down -v` wipes them.

If you use your own PostgreSQL instead, apply the migration by hand:

```bash
psql "$DATABASE_URL" -f migrations/001_create_tasks.sql
```

### Configuration

| Variable       | Required | Default | Purpose                          |
| -------------- | -------- | ------- | -------------------------------- |
| `DATABASE_URL`     | task service | n/a    | PostgreSQL connection string |
| `TASK_SERVICE_URL` | gateway      | n/a    | Private task-service URL     |
| `PORT`             | no           | varies | Service listening port       |

## API

Base path: `/api/v1`

| Method | Endpoint                | Purpose                         | Success |
| ------ | ----------------------- | ------------------------------- | ------- |
| POST   | `/tasks`                | Create a task                   | 201     |
| GET    | `/tasks`                | List tasks (newest first)       | 200     |
| GET    | `/tasks/{id}`           | Get a task by ID                | 200     |
| PUT    | `/tasks/{id}`           | Update title and description    | 200     |
| PATCH  | `/tasks/{id}/complete`  | Toggle completed / incomplete   | 200     |
| DELETE | `/tasks/{id}`           | Delete a task                   | 204     |

A task looks like this:

```json
{
  "id": "5b6f0d0e-8c1a-4c3e-9d57-0f6f2b3e9a11",
  "title": "Finish Go task manager",
  "description": "Implement the basic CRUD API",
  "completed": false,
  "created_at": "2026-10-30T20:00:00Z",
  "updated_at": "2026-10-30T20:00:00Z"
}
```

Rules:

- `title` is required, 1–200 characters after trimming whitespace.
- `description` is optional. `PUT` replaces both fields, so omitting
  `description` clears it.
- Unknown JSON fields and request bodies over 1 MiB are rejected.

### Errors

Every error is JSON in the form `{"error": "message"}`.

| Status | When                                                              |
| ------ | ----------------------------------------------------------------- |
| 400    | Invalid JSON, failed validation, or a task ID that is not a UUID  |
| 404    | No task with that ID, or an unknown route                         |
| 405    | Wrong HTTP method for a known path (an `Allow` header is sent)    |
| 413    | Request body too large                                            |
| 500    | Unexpected failure (details are logged, not returned)             |

### curl examples

```bash
# Create
curl -i -X POST http://localhost:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Finish Go task manager","description":"Implement the basic CRUD API"}'

# List
curl http://localhost:8080/api/v1/tasks

# Use the id from the create response
ID=5b6f0d0e-8c1a-4c3e-9d57-0f6f2b3e9a11

# Get one
curl http://localhost:8080/api/v1/tasks/$ID

# Update
curl -X PUT http://localhost:8080/api/v1/tasks/$ID \
  -H 'Content-Type: application/json' \
  -d '{"title":"Finish Go task manager (v1)","description":"CRUD done, tests next"}'

# Toggle complete / incomplete (call again to undo)
curl -X PATCH http://localhost:8080/api/v1/tasks/$ID/complete

# Delete
curl -i -X DELETE http://localhost:8080/api/v1/tasks/$ID
```

## Tests

```bash
go test ./...
```

Service and handler tests use an in-memory repository, so they need no
database. The PostgreSQL repository test runs only when `TEST_DATABASE_URL` is
set (and the migration has been applied):

```bash
docker compose up -d db
TEST_DATABASE_URL="postgres://taskflow:taskflow@localhost:5432/taskflow?sslmode=disable" go test ./...
```

## Project structure

```
taskflow/
├── cmd/api/main.go               # public API gateway and reverse proxy
├── cmd/task-service/main.go      # task service wiring, DB, router, shutdown
├── frontend/                     # React + TypeScript task-management UI
│   ├── src/App.tsx               # UI state, task actions, layout
│   ├── src/api.ts                # typed REST client
│   ├── src/styles.css            # responsive visual design
│   ├── Dockerfile                # production static build
│   └── nginx.conf                # SPA fallback and API proxy
├── internal/
│   ├── task/
│   │   ├── model.go              # Task type, input type, error values
│   │   ├── handler.go            # HTTP: routing, JSON in/out, status codes
│   │   ├── service.go            # business rules: validation, IDs, timestamps
│   │   ├── repository.go         # Repository interface + PostgreSQL implementation
│   │   └── uuid.go               # UUID v4 generation and ID format check
│   └── database/postgres.go      # opens and pings the connection pool
├── migrations/001_create_tasks.sql
├── docker-compose.yml
├── Dockerfile
├── .env.example
└── README.md
```

The deployment has two service boundaries: `gateway → task-service`, and the
task service internally uses `handler → service → repository → PostgreSQL`.
The task service depends on the `Repository` interface, which is what lets the
tests swap in a fake.

## Design notes

- **Small microservices deployment.** The gateway owns public ingress and
  forwarding, while the task service owns task behavior and its database. This
  is intentionally a small split that leaves room for future independent
  services such as notifications or identity.
- **Standard library first.** Routing uses `net/http` (Go 1.22 patterns), JSON
  uses `encoding/json`, and UUIDs come from `crypto/rand`.
- **One dependency: `github.com/jackc/pgx/v5`.** `database/sql` needs a
  PostgreSQL driver, and pgx is the maintained one. It is used only through
  `database/sql`.
- **Parameterised SQL only.** Every query uses `$1, $2, ...` placeholders.
- **Atomic toggle.** `PATCH .../complete` flips the flag with a single
  `UPDATE ... SET completed = NOT completed`, so concurrent requests cannot
  overwrite each other.
- **Malformed IDs are caught before the database**, so they return a clean 400
  instead of a driver error.
- **Migrations are plain SQL.** Compose runs them on first start; there is no
  migration tool to learn or maintain for a single table.
