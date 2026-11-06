FROM node:22-slim AS web

WORKDIR /web
COPY ./web/package.json ./web/package-lock.json /web/
RUN npm ci --no-fund --no-audit

COPY ./web /web
RUN npm run build -- --outDir dist

FROM ghcr.io/astral-sh/uv:python3.13-trixie

COPY ./cairn/pyproject.toml /cairn/pyproject.toml
COPY ./cairn/uv.lock /cairn/uv.lock
WORKDIR /cairn
RUN uv sync --frozen --no-install-project -i https://mirrors.aliyun.com/pypi/simple/

COPY ./cairn /cairn
RUN uv sync --frozen -i https://mirrors.aliyun.com/pypi/simple/

COPY --from=web /web/dist /cairn/src/cairn/server/static

ENV TZ=Asia/Shanghai
