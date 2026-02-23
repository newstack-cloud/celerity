FROM dhi.io/golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /app/celerity-local-events .

FROM dhi.io/debian-base:trixie AS production

COPY --from=builder /app/celerity-local-events /usr/local/bin/celerity-local-events

USER nobody

ENTRYPOINT ["celerity-local-events"]
