# =============================================================================
# Celerity Core Runtime - Multi-Target Dockerfile (Docker Hardened Images)
#
# The core runtime serves HTTP, WebSockets and queue consumers, and hands the
# events it takes to a handlers executable over the Celerity IPC protocol. This
# image is a base image: it carries the runtime and no handlers. Build your own
# image FROM it, add your compiled handlers executable, and point
# CELERITY_HANDLERS_EXECUTABLE at it.
#
#   FROM ghcr.io/newstack-cloud/celerity-runtime-core:1.0.0
#   COPY --chown=nobody handlers /opt/celerity/app/handlers
#   COPY --chown=nobody blueprint.yaml /opt/celerity/app/blueprint.yaml
#
# Targets:
#   runtime  — Production (no shell, no package manager)
#   dev      — Development (shell and the usual debugging tools)
#
# The build context is the repository root, because the runtime crates the app
# depends on live under libs/runtime:
#   docker build -f apps/runtime/core/Dockerfile --target runtime .
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Build the runtime binary
# -----------------------------------------------------------------------------
FROM dhi.io/rust:1.98 AS builder

WORKDIR /build

# The path dependencies have to be in place before the manifest resolves, so
# the whole of libs/runtime is copied rather than just the crates named in
# Cargo.toml — those crates depend on their siblings in turn.
# Copied as the image's own non-root user (uid 65532), which owns none of the
# default /build workdir, so that cargo can write its target directory.
COPY --chown=65532:65532 libs/runtime/ libs/runtime/
COPY --chown=65532:65532 apps/runtime/core/Cargo.toml apps/runtime/core/Cargo.lock apps/runtime/core/
COPY --chown=65532:65532 apps/runtime/core/src/ apps/runtime/core/src/

WORKDIR /build/apps/runtime/core

RUN cargo build --release --locked

# Created here rather than in the later stages, which have no shell to run
# mkdir in, and copied out with the ownership the runtime needs.
RUN mkdir -p /tmp/skeleton/app /tmp/skeleton/socket

# -----------------------------------------------------------------------------
# Stage 2: Production runtime (no shell, no package manager)
# -----------------------------------------------------------------------------
FROM dhi.io/debian-base:trixie AS runtime

COPY --from=builder /build/apps/runtime/core/target/release/celerity-runtime-core \
    /usr/local/bin/celerity-runtime-core

# The directory the handler stream socket is bound in, and the one a derivative
# image puts its handlers executable and blueprint in. Both are owned by the
# user the runtime runs as, since it binds the socket and reads the blueprint.
COPY --from=builder --chown=nobody /tmp/skeleton/app/ /opt/celerity/app/
COPY --from=builder --chown=nobody /tmp/skeleton/socket/ /var/run/celerity/

ENV CELERITY_RUNTIME_CALL_MODE=ipc \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false \
    CELERITY_BLUEPRINT=/opt/celerity/app/blueprint.yaml \
    CELERITY_HANDLERS_EXECUTABLE=/opt/celerity/app/handlers \
    CELERITY_RUNTIME_SOCKET=/var/run/celerity/runtime.sock

EXPOSE 8080

USER nobody

ENTRYPOINT ["celerity-runtime-core"]

# -----------------------------------------------------------------------------
# Stage 3: Development image (has a shell, for getting inside a running app)
# -----------------------------------------------------------------------------
FROM dhi.io/debian-base:trixie-dev AS dev

COPY --from=builder /build/apps/runtime/core/target/release/celerity-runtime-core \
    /usr/local/bin/celerity-runtime-core

COPY --from=builder --chown=nobody /tmp/skeleton/app/ /opt/celerity/app/
COPY --from=builder --chown=nobody /tmp/skeleton/socket/ /var/run/celerity/

# The same defaults as the production image. The dev image differs in having a
# shell and the tools to debug with, not in how the runtime behaves; platform is
# a deploy target rather than a build flavour, and `local` selects different
# consumers. The FFI dev images (e.g. Node.js) behave this way too.
ENV CELERITY_RUNTIME_CALL_MODE=ipc \
    CELERITY_SERVER_PORT=8080 \
    CELERITY_RUNTIME_PLATFORM=other \
    CELERITY_SERVER_LOOPBACK_ONLY=false \
    CELERITY_BLUEPRINT=/opt/celerity/app/blueprint.yaml \
    CELERITY_HANDLERS_EXECUTABLE=/opt/celerity/app/handlers \
    CELERITY_RUNTIME_SOCKET=/var/run/celerity/runtime.sock

EXPOSE 8080

USER nobody

ENTRYPOINT ["celerity-runtime-core"]
