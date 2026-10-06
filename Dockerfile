FROM oven/bun:1.3.14 AS management-builder

WORKDIR /app/web/management-center

COPY web/management-center/package.json web/management-center/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/management-center/ ./

ARG VERSION=dev
RUN VERSION="${VERSION}" bun run build \
    && test -f dist/index.html \
    && test "$(find dist -type f | wc -l)" -eq 1

FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential git && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./xinu-proxy-api ./cmd/server/

FROM debian:bookworm

RUN apt-get update && apt-get install -y --no-install-recommends tzdata ca-certificates && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /xinuproxyapi/static

COPY --from=builder /app/xinu-proxy-api /xinuproxyapi/xinu-proxy-api
COPY --from=management-builder /app/web/management-center/dist/index.html /xinuproxyapi/static/management.html
COPY --from=builder /app/config.example.yaml /xinuproxyapi/config.example.yaml

WORKDIR /xinuproxyapi

EXPOSE 20128

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./xinu-proxy-api"]
