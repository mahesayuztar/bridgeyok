FROM node:24.20.0-bookworm-slim AS builder

WORKDIR /workspace

ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json
COPY packages/contracts/package.json ./packages/contracts/package.json
RUN --mount=type=cache,id=bridgeyok-pnpm-store,target=/pnpm/store \
    corepack enable && \
    corepack pnpm install --frozen-lockfile --store-dir=/pnpm/store

COPY apps/web ./apps/web
COPY packages/contracts ./packages/contracts
RUN corepack pnpm --filter @bridgeyok/web build

FROM node:24.20.0-bookworm-slim AS runtime

ARG RELEASE_ID=local

LABEL io.bridgeyok.release="${RELEASE_ID}"

ENV NODE_ENV=production

WORKDIR /workspace

COPY --from=builder --chown=node:node /workspace/apps/web/.next/standalone ./
COPY --from=builder --chown=node:node /workspace/apps/web/.next/static ./apps/web/.next/static

USER node
WORKDIR /workspace/apps/web

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
    CMD ["node", "-e", "fetch('http://127.0.0.1:3000').then(response => { if (!response.ok) process.exit(1) }).catch(() => process.exit(1))"]

ENTRYPOINT ["node", "server.js"]
