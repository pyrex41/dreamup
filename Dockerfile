ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

# Build the server binary from cmd/server
RUN go build -v -o /run-app ./cmd/server


# Build frontend
FROM node:20-bookworm as frontend-builder

WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build


FROM debian:bookworm

# Install Chrome dependencies for headless browser
RUN apt-get update && apt-get install -y \
    ca-certificates \
    chromium \
    chromium-driver \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /data /var/www/html

COPY --from=builder /run-app /usr/local/bin/
COPY --from=frontend-builder /app/dist /var/www/html

# Set environment variables
ENV DB_PATH=/data/dreamup.db
ENV PORT=8080
ENV STATIC_DIR=/var/www/html

EXPOSE 8080

CMD ["run-app"]
