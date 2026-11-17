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

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY ./server/go.mod ./server/go.sum ./
RUN go mod download
COPY ./server ./
COPY --from=web /web/dist ./internal/api/static/
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /cairn ./cmd/cairn

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=server /cairn /usr/local/bin/cairn
COPY --from=dispatcher /cairn-dispatcher /usr/local/bin/cairn-dispatcher

ENV TZ=Asia/Shanghai
WORKDIR /cairn
EXPOSE 8000
CMD ["cairn", "serve", "--host", "0.0.0.0"]
