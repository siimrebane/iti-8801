# ITI8801 Notes: one image, two modes (MODE=frontend | MODE=api).
#
# Three stages. The first builds the Vue page, the second compiles the Go
# binary with the page embedded, the third is the image that ships: the
# binary alone on an empty base. Nothing else is in it, which is why it is
# small and why there is nothing in it to scan.

# --- 1. the frontend: Vue source -> static files -----------------------------
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build            # writes /src/dist

# --- 2. the backend: Go source + static files -> one static binary -----------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY --from=frontend /src/dist ./dist
ARG VERSION=dev
ARG COMMIT=local
ARG BEACON_SECRET=dev-secret
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.secret=${BEACON_SECRET}" -o /notes .

# --- 3. what ships -----------------------------------------------------------
FROM scratch
COPY --from=build /notes /notes
# The api needs TLS roots only if DATABASE_URL uses sslmode=verify-full;
# managed databases on both clouds work with sslmode=require, which does not.
EXPOSE 80 8080
ENTRYPOINT ["/notes"]
