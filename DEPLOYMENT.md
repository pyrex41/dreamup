# DreamUp QA Agent - Deployment Guide

## Fly.io Deployment

### Prerequisites

1. **Install Fly CLI**:
   ```bash
   curl -L https://fly.io/install.sh | sh
   ```

2. **Login to Fly.io**:
   ```bash
   fly auth login
   ```

### Initial Setup (One-time)

1. **Create the app** (if not already created):
   ```bash
   fly launch --no-deploy
   ```

2. **Create persistent volume for SQLite database**:
   ```bash
   fly volumes create dreamup_data --size 1 --region dfw
   ```

   **Important**: For production high availability, create a second volume:
   ```bash
   fly volumes create dreamup_data --size 1 --region dfw
   ```

3. **Set secrets** (API keys):
   ```bash
   fly secrets set OPENAI_API_KEY="sk-..."
   ```

### Deploy

```bash
fly deploy
```

This will:
1. Build the Docker image with Go 1.24 and Chromium
2. Push the image to Fly.io registry
3. Deploy to Dallas (DFW) region
4. Mount the persistent volume at `/data`
5. Start the server on port 8080

### Verify Deployment

```bash
# Check app status
fly status

# View logs
fly logs

# Open in browser
fly open

# SSH into machine
fly ssh console

# Check database
fly ssh console -C "ls -lah /data"
```

### Configuration

The app is configured via `fly.toml`:

- **Region**: Dallas, Texas (DFW)
- **Memory**: 4GB (needed for Chrome browser instances)
- **CPUs**: 2 shared cores
- **Auto-scaling**: Stops when idle, starts on request
- **Database**: SQLite at `/data/dreamup.db` (persistent volume)

### Environment Variables

Set via `fly secrets set`:

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | Yes | OpenAI API key for GPT-4 Vision |
| `DB_PATH` | No | Database path (default: `/data/dreamup.db`) |
| `PORT` | No | HTTP port (default: `8080`) |

### Monitoring

```bash
# View real-time logs
fly logs

# Monitor metrics
fly dashboard

# Check machine health
fly checks list
```

### Scaling

**Vertical Scaling** (increase resources):
```bash
fly scale memory 8192  # 8GB
fly scale vm shared-cpu-4x  # 4 CPUs
```

**Horizontal Scaling** (multiple machines):
```bash
fly scale count 2 --region dfw
fly scale count 1 --region ord  # Add Chicago region
```

**Note**: Each machine needs its own volume for SQLite. For multi-region:
- Use read replicas (LiteFS)
- Or switch to PostgreSQL for shared database

### Backups

**Manual backup**:
```bash
fly ssh console -C "sqlite3 /data/dreamup.db .dump" > backup-$(date +%Y%m%d).sql
```

**Scheduled backups** (via Fly snapshots):
```bash
fly volumes snapshots list dreamup_data
```

Fly.io automatically takes volume snapshots (retained for 5 days).

### Troubleshooting

**Build fails with "no Go files"**:
- Check Dockerfile builds from `./cmd/server`
- Verify `.dockerignore` doesn't exclude Go source files

**Chrome fails to start**:
- Ensure Dockerfile installs `chromium` and `chromium-driver`
- Check memory allocation (minimum 2GB recommended)
- Add `--no-sandbox` flag (already configured in `browser.go`)

**Database locked errors**:
- Check volume is mounted at `/data`
- Verify `DB_PATH=/data/dreamup.db` environment variable
- Ensure SQLite uses WAL mode (configured in `database.go`)

**App crashes on startup**:
```bash
fly logs  # Check error messages
fly ssh console  # SSH into machine to debug
```

**502 Bad Gateway**:
- Check app is listening on `0.0.0.0:8080` (not `localhost`)
- Verify `PORT` environment variable matches `fly.toml`

### Costs

Fly.io pricing (as of 2024):

- **Free tier**:
  - 3 shared-cpu-1x VMs (256MB RAM)
  - 3GB persistent volume storage
  - 160GB outbound transfer

- **Paid tier** (DreamUp configuration):
  - 4GB shared-cpu-2x VM: ~$24/month
  - 1GB volume: $0.15/month
  - Outbound transfer: $0.02/GB (after free 100GB)

**Estimated monthly cost**: $25-30 for 24/7 operation

**Cost optimization**:
- Enable `auto_stop_machines = true` to stop when idle (saves ~70%)
- Use `min_machines_running = 0` to scale to zero
- Typical cost with auto-stop: $5-10/month

### Production Checklist

- [ ] Set `OPENAI_API_KEY` secret
- [ ] Create 2+ volumes for redundancy
- [ ] Enable HTTPS (automatic with Fly.io)
- [ ] Set up monitoring alerts
- [ ] Configure backup strategy
- [ ] Test auto-scaling behavior
- [ ] Add custom domain (optional)
- [ ] Set up CI/CD pipeline (GitHub Actions)

### Custom Domain (Optional)

```bash
# Add custom domain
fly certs create qa.example.com

# Add DNS record (from your DNS provider)
# CNAME: qa.example.com -> dreamup.fly.dev

# Verify
fly certs check qa.example.com
```

### CI/CD with GitHub Actions

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Fly.io

on:
  push:
    branches: [main, master]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: superfly/flyctl-actions/setup-flyctl@master
      - run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

Get your Fly API token:
```bash
fly tokens create deploy
```

Add to GitHub repository secrets as `FLY_API_TOKEN`.

---

## Alternative Deployment Options

### Docker Compose (Self-hosted)

```yaml
version: '3.8'

services:
  dreamup:
    build: .
    ports:
      - "8080:8080"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - DB_PATH=/data/dreamup.db
    volumes:
      - dreamup-data:/data
    restart: unless-stopped

volumes:
  dreamup-data:
```

Deploy:
```bash
docker-compose up -d
```

### AWS Lambda

See [ARCHITECTURE.md](ARCHITECTURE.md#4-aws-lambda-deployment) for Lambda deployment guide.

### Kubernetes

Create deployment manifests in `deployment/k8s/`:
- `deployment.yaml` - Pod configuration
- `service.yaml` - Load balancer
- `persistent-volume.yaml` - SQLite storage
- `secret.yaml` - API keys

Deploy:
```bash
kubectl apply -f deployment/k8s/
```

---

## Rollback

**Fly.io rollback**:
```bash
# List releases
fly releases

# Rollback to previous version
fly releases rollback <version>

# Or deploy specific version
fly deploy --image registry.fly.io/dreamup:<version>
```

**Volume restore**:
```bash
# List snapshots
fly volumes snapshots list dreamup_data

# Restore from snapshot
fly volumes restore dreamup_data --snapshot <snapshot-id>
```

---

## Health Checks

The app exposes `/health` endpoint:

```bash
curl https://dreamup.fly.dev/health
```

Response:
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "time": "2025-11-04T22:00:00Z"
}
```

Fly.io automatically monitors this endpoint.

---

## Support

- **Fly.io Docs**: https://fly.io/docs
- **Community Forum**: https://community.fly.io
- **Status**: https://status.fly.io

---

**Last Updated**: November 4, 2025
