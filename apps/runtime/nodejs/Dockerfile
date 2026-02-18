# =============================================================================
# Celerity Node.js Runtime — Multi-Target Dockerfile (Docker Hardened Images)
#
# Targets:
#   runtime  — Production (distroless, no shell)
#   dev      — Development (shell, SWC + pino-pretty, file watching)
#
# Build:
#   docker build --target runtime -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:1.0.0 .
#   docker build --target dev -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:dev-1.0.0 .
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Install production dependencies only
# -----------------------------------------------------------------------------
FROM dhi.io/node:22-debian13-dev AS deps-prod

WORKDIR /opt/celerity

COPY package.json .yarnrc.yml yarn.lock ./

RUN corepack enable && corepack prepare yarn@4.3.1 --activate \
    && yarn install --immutable 2>/dev/null || yarn install

# Remove devDependencies after install (yarn doesn't support --production with PnP/node-modules)
RUN yarn workspaces focus --production 2>/dev/null || true

# -----------------------------------------------------------------------------
# Stage 2: Install all dependencies (prod + dev)
# -----------------------------------------------------------------------------
FROM dhi.io/node:22-debian13-dev AS deps-dev

WORKDIR /opt/celerity

COPY package.json .yarnrc.yml yarn.lock ./

RUN corepack enable && corepack prepare yarn@4.3.1 --activate \
    && yarn install --immutable 2>/dev/null || yarn install

# -----------------------------------------------------------------------------
# Stage 3: Build-time prep (needs shell for mkdir, generate-manifest, etc.)
# -----------------------------------------------------------------------------
FROM dhi.io/node:22-debian13-dev AS build

WORKDIR /opt/celerity

# Copy production node_modules from deps stage
COPY --from=deps-prod /opt/celerity/node_modules ./node_modules
COPY --from=deps-prod /opt/celerity/package.json ./package.json
COPY --from=deps-prod /opt/celerity/.yarnrc.yml ./.yarnrc.yml

# Copy runtime host files
COPY index.mjs register-hooks.mjs sdk-resolver.mjs sdk-compat-check.mjs generate-manifest.mjs tsconfig.json ./

# Generate runtime-manifest.json with installed SDK versions
RUN node generate-manifest.mjs

# Create app directory for derivative images
RUN mkdir -p /opt/celerity/app

# -----------------------------------------------------------------------------
# Stage 4: Production runtime (distroless — no shell, no package manager)
# -----------------------------------------------------------------------------
FROM dhi.io/node:22-debian13 AS runtime

WORKDIR /opt/celerity

# Copy everything prepared in the build stage
COPY --from=build /opt/celerity/ ./

# Default environment variables
ENV NODE_ENV=production \
    CELERITY_RUNTIME_CALL_MODE=ffi \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false

EXPOSE 8080

USER node

# Direct node invocation — no shell needed
ENTRYPOINT ["node", \
    "--import", "./register-hooks.mjs", \
    "--import", "@celerity-sdk/telemetry/setup", \
    "index.mjs"]

# -----------------------------------------------------------------------------
# Stage 5: Development image (has shell, SWC for TS decorators, pino-pretty)
# -----------------------------------------------------------------------------
FROM dhi.io/node:22-debian13-dev AS dev

WORKDIR /opt/celerity

# Copy full node_modules (includes SWC, pino-pretty)
COPY --from=deps-dev /opt/celerity/node_modules ./node_modules
COPY --from=deps-dev /opt/celerity/package.json ./package.json
COPY --from=deps-dev /opt/celerity/.yarnrc.yml ./.yarnrc.yml

# Copy runtime host files
COPY index.mjs register-hooks.mjs sdk-resolver.mjs sdk-compat-check.mjs generate-manifest.mjs tsconfig.json ./
COPY entrypoint.sh ./
RUN chmod +x entrypoint.sh

# Generate runtime-manifest.json with installed SDK versions
RUN node generate-manifest.mjs

# Create app directory for derivative images
RUN mkdir -p /opt/celerity/app

# Default environment variables
ENV NODE_ENV=development \
    CELERITY_RUNTIME_CALL_MODE=ffi \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false

EXPOSE 8080

USER node

ENTRYPOINT ["./entrypoint.sh"]
CMD ["dev"]
