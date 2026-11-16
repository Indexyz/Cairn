FROM node:22-slim AS web

WORKDIR /web
COPY ./web/package.json ./web/package-lock.json /web/
RUN npm ci --no-fund --no-audit

COPY ./web /web
RUN npm run build -- --outDir dist

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS dispatcher

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY ./dispatcher/go.mod ./dispatcher/go.sum ./
RUN go mod download
COPY ./dispatcher ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /cairn-dispatcher ./cmd/cairn-dispatcher

FROM ghcr.io/astral-sh/uv:python3.13-trixie

COPY --from=dispatcher /cairn-dispatcher /usr/local/bin/cairn-dispatcher

COPY ./cairn/pyproject.toml /cairn/pyproject.toml
COPY ./cairn/uv.lock /cairn/uv.lock
WORKDIR /cairn
RUN uv sync --frozen --no-install-project -i https://mirrors.aliyun.com/pypi/simple/

COPY ./cairn /cairn
RUN uv sync --frozen -i https://mirrors.aliyun.com/pypi/simple/

COPY --from=web /web/dist /cairn/src/cairn/server/static

ENV TZ=Asia/Shanghai
