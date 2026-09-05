FROM oven/bun:1.3-alpine AS ui

WORKDIR /src/web

COPY web/package.json .
COPY web/bun.lock .
RUN bun install --frozen-lockfile

COPY web/ .

RUN bun run build

# Stage 2: Go build
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./

RUN go mod download

COPY . .
COPY --from=ui /src/internal/web/dist ./internal/web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 3:
FROM golang:1.26-alpine AS migrate
WORKDIR /src
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
COPY migrations ./migrations
ENTRYPOINT ["goose", "-dir", "migrations"]

# Stage 4:
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 10001 opd
USER opd

COPY --from=build /server /server

EXPOSE 8080
ENTRYPOINT ["/server"]