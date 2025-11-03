# DreamUp QA Agent - Terraform Deployment

This directory contains Terraform configuration for deploying the DreamUp QA Agent to AWS Lambda.

## Prerequisites

- Terraform >= 1.0
- AWS CLI configured with appropriate credentials
- Built Lambda deployment package (`lambda-deployment.zip`)

## Quick Start

1. **Build the Lambda function**:
   ```bash
   cd ../..
   ./scripts/build-lambda.sh
   ```

2. **Initialize Terraform**:
   ```bash
   cd deployment/terraform
   terraform init
   ```

3. **Create `terraform.tfvars`**:
   ```hcl
   aws_region      = "us-east-1"
   environment     = "prod"
   function_name   = "dreamup-qa-agent"
   s3_bucket_name  = "dreamup-qa-artifacts-prod"
   openai_api_key  = "sk-..."
   enable_function_url = false
   ```

4. **Plan deployment**:
   ```bash
   terraform plan
   ```

5. **Deploy**:
   ```bash
   terraform apply
   ```

## Resources Created

- **Lambda Function**: `dreamup-qa-agent`
  - Runtime: Custom (Go binary)
  - Timeout: 5 minutes
  - Memory: 2048 MB
  - Handler: `bootstrap`

- **S3 Bucket**: For storing test artifacts
  - Screenshots
  - Console logs
  - Reports

- **IAM Role**: Lambda execution role with:
  - S3 read/write permissions
  - CloudWatch Logs permissions

- **CloudWatch Log Group**: 7-day retention

- **Lambda Function URL** (optional): HTTP endpoint for invocation

## Configuration Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `aws_region` | AWS region | `us-east-1` |
| `environment` | Environment name | `dev` |
| `function_name` | Lambda function name | `dreamup-qa-agent` |
| `s3_bucket_name` | S3 bucket name | `dreamup-qa-artifacts` |
| `openai_api_key` | OpenAI API key | (required) |
| `enable_function_url` | Enable HTTP access | `false` |

## Invoking the Lambda Function

### Via AWS CLI

```bash
aws lambda invoke \
  --function-name dreamup-qa-agent \
  --payload '{"game_url":"https://example.com/game","upload_to_s3":true}' \
  response.json

cat response.json
```

### Via Function URL (if enabled)

```bash
curl -X POST https://your-function-url.lambda-url.us-east-1.on.aws/ \
  -H "Content-Type: application/json" \
  -d '{"game_url":"https://example.com/game","upload_to_s3":true}'
```

### Event Schema

```json
{
  "game_url": "https://example.com/game",
  "upload_to_s3": true,
  "bucket_name": "custom-bucket",
  "timeout": 280,
  "metadata": {
    "test_env": "production",
    "build_id": "12345"
  }
}
```

### Response Schema

```json
{
  "success": true,
  "report_id": "uuid",
  "report_url": "https://bucket.s3.amazonaws.com/reports/uuid/report.json",
  "status": "passed",
  "summary": {
    "status": "passed",
    "passed_checks": ["Game loads successfully"],
    "failed_checks": [],
    "critical_issues": []
  },
  "duration_seconds": 12.5
}
```

## Updating the Function

1. Rebuild the Lambda package:
   ```bash
   ./scripts/build-lambda.sh
   ```

2. Apply changes:
   ```bash
   terraform apply
   ```

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**Warning**: This will delete the S3 bucket and all artifacts!

## Cost Estimates

- **Lambda**: ~$0.20 per 1000 invocations (with 2GB memory, 30s avg duration)
- **S3**: ~$0.023 per GB/month storage
- **CloudWatch Logs**: ~$0.50 per GB ingested

Typical cost: **$5-20/month** for moderate usage (100-500 tests/day)

## Security Considerations

1. **API Keys**: Store `OPENAI_API_KEY` in AWS Secrets Manager instead of env vars
2. **Function URL**: Use `AWS_IAM` authorization instead of `NONE`
3. **S3 Bucket**: Enable encryption and restrict public access
4. **VPC**: Deploy Lambda in VPC for additional network isolation

## Monitoring

- **CloudWatch Logs**: `/aws/lambda/dreamup-qa-agent`
- **Metrics**: Lambda duration, errors, invocations
- **Alarms**: Set up alerts for error rates > 5%
