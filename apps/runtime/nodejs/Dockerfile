# =============================================================================
# Celerity Node.js Runtime — Multi-Target Dockerfile
#
# Targets:
#   runtime  — Production base image (ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:1.0.0)
#   dev      — Dev image with tsx + pino-pretty (ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:dev-1.0.0)
#
# Build:
#   docker build --target runtime -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:1.0.0 .
#   docker build --target dev -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-22:dev-1.0.0 .
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Install production dependencies only
# -----------------------------------------------------------------------------
FROM node:22-slim AS deps-prod

WORKDIR /opt/celerity

COPY package.json .yarnrc.yml yarn.lock ./

RUN corepack enable && corepack prepare yarn@4.3.1 --activate \
    && yarn install --immutable 2>/dev/null || yarn install

# Remove devDependencies after install (yarn doesn't support --production with PnP/node-modules)
RUN yarn workspaces focus --production 2>/dev/null || true

# -----------------------------------------------------------------------------
# Stage 2: Install all dependencies (prod + dev)
# -----------------------------------------------------------------------------
FROM node:22-slim AS deps-dev

WORKDIR /opt/celerity

COPY package.json .yarnrc.yml yarn.lock ./

RUN corepack enable && corepack prepare yarn@4.3.1 --activate \
    && yarn install --immutable 2>/dev/null || yarn install

# -----------------------------------------------------------------------------
# Stage 3: Production runtime image
# -----------------------------------------------------------------------------
FROM node:22-slim AS runtime

# Security: run as non-root user
RUN groupadd --gid 1001 celerity \
    && useradd --uid 1001 --gid celerity --shell /bin/bash --create-home celerity

WORKDIR /opt/celerity

# Copy production node_modules from deps stage
COPY --from=deps-prod /opt/celerity/node_modules ./node_modules
COPY --from=deps-prod /opt/celerity/package.json ./package.json
COPY --from=deps-prod /opt/celerity/.yarnrc.yml ./.yarnrc.yml

# Copy runtime host files
COPY index.mjs register-hooks.mjs sdk-resolver.mjs sdk-compat-check.mjs generate-manifest.mjs ./
COPY entrypoint.sh ./
RUN chmod +x entrypoint.sh

# Generate runtime-manifest.json with installed SDK versions
RUN node generate-manifest.mjs

# Create app directory for derivative images
RUN mkdir -p /opt/celerity/app \
    && chown -R celerity:celerity /opt/celerity

# Default environment variables
ENV NODE_ENV=production \
    CELERITY_RUNTIME_CALL_MODE=ffi \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false

EXPOSE 8080

USER celerity

ENTRYPOINT ["./entrypoint.sh"]
CMD ["start"]

# -----------------------------------------------------------------------------
# Stage 4: Development image (extends production, adds dev tooling)
# -----------------------------------------------------------------------------
FROM runtime AS dev

USER root

# Replace node_modules with full deps (includes tsx, pino-pretty)
COPY --from=deps-dev /opt/celerity/node_modules ./node_modules

RUN chown -R celerity:celerity /opt/celerity

USER celerity

ENV NODE_ENV=development

CMD ["dev"]
