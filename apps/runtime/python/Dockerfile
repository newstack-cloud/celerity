# =============================================================================
# Celerity Python Runtime — Multi-Target Dockerfile (Docker Hardened Images)
#
# Targets:
#   runtime  — Production (minimal, no dev tools)
#   dev      — Development (watchfiles auto-reload)
#
# Build:
#   docker build --target runtime -t ghcr.io/newstack-cloud/celerity-runtime-python-3-13:1.0.0 .
#   docker build --target dev -t ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-1.0.0 .
#
# Dependency Resolution:
#   Python's import system uses a single sys.modules namespace, so unlike
#   Node.js there is no duplicate-symbol problem from multiple package copies.
#   Instead, the runtime and app dependencies are installed into the same venv:
#
#   1. The runtime's pyproject.toml brings celerity-sdk[runtime] (core + PyO3).
#   2. Derivative images install the app's dependencies (e.g. celerity-sdk[cache],
#      celerity-sdk[resources-sql]) into the same venv.
#   3. Resource layers that are not installed are silently skipped at startup
#      (see celerity.layers.system.create_default_system_layers).
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Install production dependencies only
# -----------------------------------------------------------------------------
FROM dhi.io/python:3.13-debian13-dev AS deps-prod

WORKDIR /opt/celerity

RUN pip install uv

COPY pyproject.toml uv.lock* ./

RUN uv sync --no-dev

# -----------------------------------------------------------------------------
# Stage 2: Install all dependencies (prod + dev)
# -----------------------------------------------------------------------------
FROM dhi.io/python:3.13-debian13-dev AS deps-dev

WORKDIR /opt/celerity

RUN pip install uv

COPY pyproject.toml uv.lock* ./

RUN uv sync

# -----------------------------------------------------------------------------
# Stage 3: Build-time prep (generate manifest)
# -----------------------------------------------------------------------------
FROM dhi.io/python:3.13-debian13-dev AS build

WORKDIR /opt/celerity

# Copy production venv from deps stage
COPY --from=deps-prod /opt/celerity/.venv ./.venv

# Copy runtime host files
COPY main.py sdk_compat_check.py generate_manifest.py pyproject.toml ./

# Generate runtime-manifest.json with installed SDK versions
RUN . /opt/celerity/.venv/bin/activate && python generate_manifest.py

# Create app directory for derivative images
RUN mkdir -p /opt/celerity/app

# -----------------------------------------------------------------------------
# Stage 4: Production runtime (minimal)
#
# Derivative images (user apps) should install their app dependencies into the
# same venv so that resource extras (cache, sql, etc.) are available:
#
#   FROM ghcr.io/newstack-cloud/celerity-runtime-python-3-13:1.0.0
#   COPY app/ /opt/celerity/app/
#   RUN . /opt/celerity/.venv/bin/activate \
#       && pip install -r /opt/celerity/app/requirements.txt
# -----------------------------------------------------------------------------
FROM dhi.io/python:3.13-debian13 AS runtime

WORKDIR /opt/celerity

# Copy everything prepared in the build stage
COPY --from=build /opt/celerity/ ./

# Default environment variables
ENV CELERITY_ENV=production \
    CELERITY_RUNTIME_CALL_MODE=ffi \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false \
    VIRTUAL_ENV=/opt/celerity/.venv \
    PATH="/opt/celerity/.venv/bin:$PATH"

EXPOSE 8080

USER nonroot

ENTRYPOINT ["python", "main.py"]

# -----------------------------------------------------------------------------
# Stage 5: Development image (watchfiles for auto-reload)
#
# In dev mode, app dependencies are installed at container startup or via
# a derivative Dockerfile. The shared venv already includes the runtime's
# celerity-sdk[runtime] plus dev tools (watchfiles).
# -----------------------------------------------------------------------------
FROM dhi.io/python:3.13-debian13-dev AS dev

WORKDIR /opt/celerity

# Copy full venv (includes watchfiles)
COPY --from=deps-dev /opt/celerity/.venv ./.venv

# Copy runtime host files
COPY main.py sdk_compat_check.py generate_manifest.py pyproject.toml ./
COPY entrypoint.sh ./
RUN chmod +x entrypoint.sh

# Generate runtime-manifest.json with installed SDK versions
RUN . /opt/celerity/.venv/bin/activate && python generate_manifest.py

# Create app directory for derivative images
RUN mkdir -p /opt/celerity/app

# Default environment variables
ENV CELERITY_ENV=development \
    CELERITY_RUNTIME_CALL_MODE=ffi \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false \
    VIRTUAL_ENV=/opt/celerity/.venv \
    PATH="/opt/celerity/.venv/bin:$PATH"

EXPOSE 8080

USER nonroot

ENTRYPOINT ["./entrypoint.sh"]
CMD ["dev"]
