FROM dhi.io/golang:1.26 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /app/celerity-dev-auth .

FROM dhi.io/debian-base:trixie-dev AS production

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/celerity-dev-auth /usr/local/bin/celerity-dev-auth

USER nobody

ENTRYPOINT ["celerity-dev-auth"]
