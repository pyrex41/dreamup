ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-alpine as builder

# Install build dependencies for SQLite (CGO)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

# Build the server binary from cmd/server with SQLite support
RUN CGO_ENABLED=1 go build -v -o /run-app ./cmd/server


# Build frontend
FROM node:20-alpine as frontend-builder

WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Install Elm compiler
RUN wget -O elm.gz https://github.com/elm/compiler/releases/download/0.19.1/binary-for-linux-64-bit.gz && \
    gunzip elm.gz && \
    chmod +x elm && \
    mv elm /usr/local/bin/

# Copy only package files first
COPY frontend/package.json frontend/pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy source files (excluding node_modules due to .dockerignore)
COPY frontend/src ./src/
COPY frontend/index.html frontend/vite.config.js frontend/elm.json ./

# Build the frontend
RUN pnpm run build


FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    chromium \
    chromium-chromedriver \
    sqlite-libs \
    tzdata

# Create directories
RUN mkdir -p /data /var/www/html

COPY --from=builder /run-app /usr/local/bin/
COPY --from=frontend-builder /app/dist /var/www/html

# Set environment variables
ENV DB_PATH=/data/dreamup.db
ENV PORT=8080
ENV STATIC_DIR=/var/www/html
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/lib/chromium/

EXPOSE 8080

CMD ["run-app"]
