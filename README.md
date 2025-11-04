# DreamUp QA Agent

**Automated QA testing for web-based games using AI-powered evaluation**

[![Status](https://img.shields.io/badge/status-production--ready-green)]()
[![Go Version](https://img.shields.io/badge/go-1.24+-blue)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()

🚀 **Live Demo**: [dreamup.fly.dev](https://dreamup.fly.dev)

📚 **Documentation**: [Architecture Guide](ARCHITECTURE.md) | [Deployment Guide](DEPLOYMENT.md)

## Overview

DreamUp QA Agent is a fully automated testing system for web-based games. It uses browser automation, AI-powered visual analysis, and intelligent reporting to evaluate game quality, identify issues, and provide actionable recommendations.

### Key Features

- 🤖 **AI-Powered Evaluation**: GPT-4 Vision analyzes screenshots and gameplay
- 🌐 **Browser Automation**: Headless Chrome with chromedp
- 📸 **Evidence Collection**: Screenshots, console logs, UI detection
- 📊 **Comprehensive Reports**: JSON reports with S3 storage
- ⚡ **Serverless Ready**: AWS Lambda deployment with Terraform
- 🔄 **Error Handling**: Automatic retry with exponential backoff
- 🎯 **Smart Detection**: Automatic UI element detection

## Quick Start

### CLI Usage

```bash
# Build the CLI
go build -o qa ./cmd/qa

# Run a test
./qa test --url https://example.com/your-game

# With custom options
./qa test \
  --url https://example.com/your-game \
  --output ./results \
  --headless=true \
  --max-duration 300
```

### Configuration

Environment variables:
```bash
export OPENAI_API_KEY="sk-..."      # For AI evaluation
export S3_BUCKET_NAME="my-bucket"   # For artifact storage (optional)
export AWS_REGION="us-east-1"       # AWS region (optional)
```

Or use a config file (`config.yaml`):
```yaml
output_dir: ./qa-results
headless: true
max_duration: 300
```

## Lambda Deployment

### Build Lambda Package

```bash
./scripts/build-lambda.sh
```

This creates `lambda-deployment.zip` ready for AWS Lambda.

### Deploy with Terraform

```bash
cd deployment/terraform

# Create terraform.tfvars
cat > terraform.tfvars <<EOF
aws_region      = "us-east-1"
environment     = "prod"
s3_bucket_name  = "my-qa-artifacts"
openai_api_key  = "sk-..."
EOF

# Deploy
terraform init
terraform apply
```

### Invoke Lambda

```bash
aws lambda invoke \
  --function-name dreamup-qa-agent \
  --payload '{"game_url":"https://example.com/game","upload_to_s3":true}' \
  response.json
```

## How It Works

1. **Browser Launch**: Headless Chrome starts with optimized settings
2. **Console Monitoring**: Captures all browser console logs (errors, warnings, info)
3. **Navigation**: Loads the game URL with timeout protection
4. **UI Detection**: Automatically finds start buttons and game canvas
5. **Interaction**: Executes smart interaction plan based on detected UI
6. **Evidence Collection**: Captures screenshots at key moments
7. **AI Evaluation**: GPT-4 Vision analyzes screenshots and logs
8. **Report Generation**: Creates comprehensive JSON report
9. **S3 Upload**: Optionally uploads all artifacts to S3
10. **Results**: Returns detailed test summary with scores and issues

## Test Report Structure

```json
{
  "report_id": "uuid",
  "game_url": "https://example.com/game",
  "timestamp": "2025-11-03T10:00:00Z",
  "duration_ms": 15000,
  "score": {
    "overall_score": 85,
    "loads_correctly": true,
    "interactivity_score": 90,
    "visual_quality": 80,
    "error_severity": 10,
    "reasoning": "Game loads and functions well...",
    "issues": ["Minor console warnings"],
    "recommendations": ["Improve error handling"]
  },
  "evidence": {
    "screenshots": [...],
    "console_logs": [...],
    "log_summary": {
      "total": 5,
      "errors": 0,
      "warnings": 2
    }
  },
  "summary": {
    "status": "passed",
    "passed_checks": ["Game loads successfully", "No console errors"],
    "failed_checks": [],
    "critical_issues": []
  }
}
```

## Architecture

```
┌─────────────────┐
│   CLI / Lambda  │
└────────┬────────┘
         │
    ┌────▼─────────────────────────────┐
    │     Test Orchestration           │
    │  (Error Handling + Retry)        │
    └────┬─────────────────────────────┘
         │
    ┌────▼────────┐  ┌──────────────┐  ┌──────────────┐
    │   Browser   │  │  Evaluator   │  │   Reporter   │
    │   Agent     │  │  (GPT-4)     │  │   (S3)       │
    └─────────────┘  └──────────────┘  └──────────────┘
         │                  │                  │
    ┌────▼────────┐  ┌──────▼──────┐  ┌────────▼────────┐
    │Screenshots  │  │  AI Scores  │  │  JSON + Upload  │
    │Console Logs │  │ + Issues    │  │                 │
    │UI Detection │  │             │  │                 │
    └─────────────┘  └─────────────┘  └─────────────────┘
```

## Project Structure

```
dreamup/
├── cmd/
│   ├── qa/          # CLI application
│   └── lambda/      # AWS Lambda handler
├── internal/
│   ├── agent/       # Browser automation + interactions
│   ├── evaluator/   # AI evaluation (GPT-4 Vision)
│   └── reporter/    # Report generation + S3 upload
├── deployment/
│   └── terraform/   # Infrastructure as Code
├── scripts/
│   └── build-lambda.sh
└── log_docs/        # Project logs
```

## API Reference

### CLI Commands

```bash
qa test --url <URL>              # Run test on game URL
qa test --help                   # Show all options
qa --version                     # Show version
```

### Lambda Event

```json
{
  "game_url": "https://example.com/game",
  "upload_to_s3": true,
  "timeout": 280,
  "metadata": {
    "build_id": "12345",
    "environment": "production"
  }
}
```

### Lambda Response

```json
{
  "success": true,
  "report_id": "uuid",
  "report_url": "https://s3.amazonaws.com/...",
  "status": "passed",
  "duration_seconds": 12.5,
  "summary": { ... }
}
```

## Dependencies

- **chromedp**: Browser automation
- **go-openai**: GPT-4 Vision integration
- **aws-sdk-go-v2**: S3 upload + Lambda
- **cobra**: CLI framework
- **viper**: Configuration management
- **uuid**: Unique ID generation

## Development

### Requirements

- Go 1.24+
- Chrome/Chromium installed
- AWS credentials (for S3 upload)
- OpenAI API key (for evaluation)

### Build

```bash
# Build CLI
go build -o qa ./cmd/qa

# Build Lambda
GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap ./cmd/lambda
zip lambda-deployment.zip bootstrap

# Run tests (when available)
go test ./...
```

### Environment Setup

```bash
# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env with your API keys
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `OPENAI_API_KEY` | OpenAI API key for evaluation | Yes (for AI) | - |
| `S3_BUCKET_NAME` | S3 bucket for artifacts | No | `dreamup-qa-artifacts` |
| `AWS_REGION` | AWS region | No | `us-east-1` |
| `DREAMUP_OUTPUT_DIR` | Output directory | No | `./qa-results` |
| `DREAMUP_HEADLESS` | Headless mode | No | `true` |

### Config File (config.yaml)

```yaml
output_dir: ./qa-results
headless: true
max_duration: 300
```

## Costs

### AWS Lambda
- **Compute**: ~$0.20 per 1000 invocations (2GB, 30s avg)
- **S3**: ~$0.023 per GB/month
- **CloudWatch**: ~$0.50 per GB logs

**Typical monthly cost**: $5-20 for 100-500 tests/day

## Monitoring

- **CloudWatch Logs**: `/aws/lambda/dreamup-qa-agent`
- **Metrics**: Duration, errors, invocations
- **Alarms**: Set up for error rate > 5%

## Security

- Store API keys in AWS Secrets Manager (not env vars)
- Use IAM authorization for Lambda Function URL
- Enable S3 encryption
- Deploy Lambda in VPC for isolation

## Troubleshooting

### "Browser failed to start"
- Ensure Chrome/Chromium is installed
- Check headless mode is enabled
- Verify no-sandbox flag is set

### "OPENAI_API_KEY not found"
- Set environment variable: `export OPENAI_API_KEY="sk-..."`
- Or configure in config.yaml

### "S3 upload failed"
- Verify AWS credentials are configured
- Check S3 bucket exists and has correct permissions
- Ensure AWS_REGION is set correctly

### Lambda timeout
- Increase timeout (max 15 minutes)
- Increase memory (more memory = faster CPU)
- Optimize game load time

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [chromedp](https://github.com/chromedp/chromedp)
- AI evaluation powered by OpenAI GPT-4 Vision
- Infrastructure managed with Terraform
- CLI framework by [Cobra](https://github.com/spf13/cobra)

## Support

For issues, questions, or feature requests, please open an issue on GitHub.

---

**DreamUp QA Agent** - Automated game testing made simple 🎮🤖
