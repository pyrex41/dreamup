#!/bin/bash

# DreamUp QA Agent Frontend Deployment Script
# Usage: ./deploy.sh [environment] [bucket-name]
# Example: ./deploy.sh production dreamup-qa-frontend

set -e  # Exit on error

ENVIRONMENT=${1:-staging}
S3_BUCKET=${2:-dreamup-qa-${ENVIRONMENT}}
DISTRIBUTION_ID=${3:-}

echo "================================================"
echo "DreamUp QA Agent Frontend Deployment"
echo "================================================"
echo "Environment: $ENVIRONMENT"
echo "S3 Bucket: $S3_BUCKET"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if npm is installed
if ! command -v npm &> /dev/null; then
    echo -e "${RED}Error: npm is not installed${NC}"
    exit 1
fi

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo -e "${RED}Error: AWS CLI is not installed${NC}"
    echo "Install it with: pip install awscli"
    exit 1
fi

# Step 1: Install dependencies
echo -e "${YELLOW}Step 1: Installing dependencies...${NC}"
npm install

# Step 2: Run build
echo -e "${YELLOW}Step 2: Building for production...${NC}"
npm run build

if [ ! -d "dist" ]; then
    echo -e "${RED}Error: Build failed - dist directory not found${NC}"
    exit 1
fi

# Step 3: Display bundle sizes
echo -e "${YELLOW}Step 3: Bundle sizes:${NC}"
du -h dist/index.html
du -h dist/assets/*.js 2>/dev/null || echo "No JS bundles found"

# Step 4: Deploy to S3
echo -e "${YELLOW}Step 4: Deploying to S3 bucket: $S3_BUCKET${NC}"

# Check if bucket exists
if ! aws s3 ls "s3://$S3_BUCKET" 2>&1 > /dev/null; then
    echo -e "${YELLOW}Bucket does not exist. Creating...${NC}"
    aws s3 mb "s3://$S3_BUCKET"

    echo -e "${YELLOW}Configuring bucket for static website hosting...${NC}"
    aws s3 website "s3://$S3_BUCKET" \
        --index-document index.html \
        --error-document index.html
fi

# Sync assets with long cache
echo -e "${YELLOW}Uploading assets with cache headers...${NC}"
aws s3 sync dist/assets/ "s3://$S3_BUCKET/assets/" \
    --delete \
    --cache-control "public, max-age=31536000, immutable" \
    --acl public-read

# Upload HTML with no cache
echo -e "${YELLOW}Uploading index.html...${NC}"
aws s3 cp dist/index.html "s3://$S3_BUCKET/index.html" \
    --cache-control "public, max-age=0, must-revalidate" \
    --content-type "text/html" \
    --acl public-read

# Step 5: Invalidate CloudFront cache (if distribution ID provided)
if [ -n "$DISTRIBUTION_ID" ]; then
    echo -e "${YELLOW}Step 5: Invalidating CloudFront cache...${NC}"
    aws cloudfront create-invalidation \
        --distribution-id "$DISTRIBUTION_ID" \
        --paths "/*"
    echo -e "${GREEN}CloudFront invalidation created${NC}"
else
    echo -e "${YELLOW}Step 5: Skipping CloudFront invalidation (no distribution ID provided)${NC}"
fi

# Step 6: Display deployment URL
echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}Deployment successful!${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo "S3 Website URL: http://$S3_BUCKET.s3-website-us-east-1.amazonaws.com"
if [ -n "$DISTRIBUTION_ID" ]; then
    CLOUDFRONT_DOMAIN=$(aws cloudfront get-distribution --id "$DISTRIBUTION_ID" --query 'Distribution.DomainName' --output text)
    echo "CloudFront URL: https://$CLOUDFRONT_DOMAIN"
fi
echo ""
echo "Next steps:"
echo "1. Test the deployment at the URL above"
echo "2. Update DNS if using custom domain"
echo "3. Monitor CloudWatch metrics for errors"
echo ""
