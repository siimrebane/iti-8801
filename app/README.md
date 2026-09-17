# ITI8801 Notes — the course application

The application students deploy from week 4 on. One image, two modes; the
third tier is a managed PostgreSQL. Students do not write it; from week 5 they
build its image themselves from this source.

| Tier | Runs | Listens | Needs |
|---|---|---|---|
| frontend | `MODE=frontend` | 80 | `API_URL=http://<service private ip>:8080` |
| api | `MODE=api` | 8080 | `DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require` |
| database | managed PostgreSQL | 5432 | one database; the api creates its table |

Both modes take `DB_TARGET=<db host>:5432` (dialled and reported: the api
tier must reach it, the frontend tier must not) and `PROBE_TARGETS=host:port,…`
(each dialled and reported). Run with `--network host` like the beacon, so the
report shows the VM's addresses, not the container's.

## Endpoints

Both modes: `/health` (200 or 503), `/version`, `/metrics` (Prometheus text),
`/report` and `/hello` (the signed self-report the validator reads; `?nonce=`
adds the signature, same secret and scheme as the beacon, canonical string in
`main.go`).

api: `GET/POST /api/notes`, `GET/DELETE /api/notes/{id}`, `/api/version`,
`/api/health`. frontend: the Vue page at `/`, and everything under `/api/`
forwarded to `API_URL`.

## Build

    docker buildx build --platform linux/amd64 \
      --build-arg VERSION=a4 --build-arg COMMIT=$(git rev-parse --short=8 HEAD) \
      --build-arg BEACON_SECRET=$(cat ../beacon/SECRET) \
      -t ghcr.io/siimrebane/iti8801-notes:a4 --load .
    docker push ghcr.io/siimrebane/iti8801-notes:a4

Three stages: Node builds the Vue page, Go compiles the binary with the page
embedded (`//go:embed all:dist`), the shipped image is the binary on `scratch`
(about 17 MB). Push needs `docker login ghcr.io` with a token that has
`write:packages`; the package must be public for anonymous pulls.

## Local development

    docker run -d --name pg -e POSTGRES_PASSWORD=pw -e POSTGRES_DB=notes -p 5432:5432 postgres:16-alpine
    MODE=api DATABASE_URL=postgres://postgres:pw@localhost:5432/notes go run .
    cd frontend && npm install && npm run dev      # Vite on :5173, /api forwarded to :8080

`go build` needs `dist/` to exist (`cd frontend && npm run build`), because
the binary embeds it.
