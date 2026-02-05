# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /build/lifi-mcp .

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/lifi-mcp /lifi-mcp

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/lifi-mcp"]
CMD ["--port", "8080", "--host", "0.0.0.0", "--log-level", "info"]
