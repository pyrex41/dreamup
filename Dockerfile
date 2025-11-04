ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

# Build the server binary from cmd/server
RUN go build -v -o /run-app ./cmd/server


FROM debian:bookworm

# Install Chrome dependencies for headless browser
RUN apt-get update && apt-get install -y \
    ca-certificates \
    chromium \
    chromium-driver \
    && rm -rf /var/lib/apt/lists/*

# Create directory for SQLite database
RUN mkdir -p /data

COPY --from=builder /run-app /usr/local/bin/

# Set environment variables
ENV DB_PATH=/data/dreamup.db
ENV PORT=8080

EXPOSE 8080

CMD ["run-app"]
