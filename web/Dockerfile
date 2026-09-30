# syntax=docker/dockerfile:1
# Build from the repository root (the API client is generated from api/openapi):
#   docker build -f web/Dockerfile -t efoy-web .

FROM node:22-alpine AS deps
WORKDIR /repo/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

FROM node:22-alpine AS build
WORKDIR /repo/web
ARG EFOY_API_URL=http://core-api:8080
ARG NEXT_PUBLIC_MAP_STYLE_URL=
ENV EFOY_API_URL=$EFOY_API_URL \
    NEXT_PUBLIC_MAP_STYLE_URL=$NEXT_PUBLIC_MAP_STYLE_URL \
    NEXT_TELEMETRY_DISABLED=1
COPY api/openapi /repo/api/openapi
COPY --from=deps /repo/web/node_modules ./node_modules
COPY web/ ./
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /repo/web/.next/standalone ./
COPY --from=build --chown=node:node /repo/web/.next/static ./.next/static
USER node
EXPOSE 3000
CMD ["node", "server.js"]
