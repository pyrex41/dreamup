#!/bin/bash
# Build script for AWS Lambda deployment

set -e

echo "🏗️  Building Lambda function..."

# Build for Linux AMD64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap cmd/lambda/main.go

# Create deployment package
echo "📦 Creating deployment package..."
zip -j lambda-deployment.zip bootstrap

# Clean up
rm bootstrap

echo "✅ Lambda deployment package created: lambda-deployment.zip"
echo ""
echo "📋 Deployment instructions:"
echo "   1. Create Lambda function with Go 1.x runtime (or custom runtime)"
echo "   2. Upload lambda-deployment.zip"
echo "   3. Set handler to 'bootstrap'"
echo "   4. Configure environment variables:"
echo "      - OPENAI_API_KEY (for LLM evaluation)"
echo "      - S3_BUCKET_NAME (for artifact storage)"
echo "      - AWS_REGION (defaults to us-east-1)"
echo "   5. Set timeout to 5 minutes (300 seconds)"
echo "   6. Set memory to at least 1024 MB (2048 MB recommended)"
echo ""
echo "📝 Example Lambda event:"
echo '   {
     "game_url": "https://example.com/game",
     "upload_to_s3": true,
     "metadata": {
       "test_env": "production"
     }
   }'
