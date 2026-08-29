FROM node:24.20.0-bookworm-slim AS builder

WORKDIR /workspace

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json
COPY packages/contracts/package.json ./packages/contracts/package.json
RUN corepack enable && corepack pnpm install --frozen-lockfile

COPY apps/web ./apps/web
COPY packages/contracts ./packages/contracts
RUN corepack pnpm --filter @bridgeyok/web build

FROM node:24.20.0-bookworm-slim AS runtime

ARG RELEASE_ID=local

LABEL io.bridgeyok.release="${RELEASE_ID}"

ENV NODE_ENV=production

WORKDIR /workspace

COPY --from=builder --chown=node:node /workspace ./

USER node
WORKDIR /workspace/apps/web

EXPOSE 3000

ENTRYPOINT ["node", "node_modules/next/dist/bin/next", "start"]
