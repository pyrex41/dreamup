# DreamUp QA Agent Frontend - Deployment Guide

## Overview

The DreamUp QA Agent frontend is a static Elm application that can be deployed to any static hosting service (S3, CloudFront, Netlify, Vercel, etc.).

## Prerequisites

- Node.js 18+ and npm
- AWS CLI (for S3/CloudFront deployment)
- Production API endpoint URL

## Build for Production

```bash
cd frontend
npm install
npm run build
```

This creates optimized production files in `dist/`:
- `dist/index.html` - Main HTML file
- `dist/assets/*.js` - Minified and hashed JavaScript bundles
- Gzipped versions for all assets

## Deployment Options

### Option 1: AWS S3 + CloudFront (Recommended)

#### 1. Create S3 Bucket

```bash
aws s3 mb s3://dreamup-qa-frontend --region us-east-1
```

#### 2. Configure Bucket for Static Website Hosting

```bash
aws s3 website s3://dreamup-qa-frontend \
  --index-document index.html \
  --error-document index.html
```

#### 3. Update API Base URL

Edit `frontend/src/Main.elm` line 273:
```elm
, apiBaseUrl = "https://api.yourdomain.com"  -- Update with production API URL
```

Rebuild:
```bash
npm run build
```

#### 4. Deploy to S3

```bash
aws s3 sync dist/ s3://dreamup-qa-frontend \
  --delete \
  --cache-control "public, max-age=31536000, immutable" \
  --exclude "index.html"

aws s3 cp dist/index.html s3://dreamup-qa-frontend/index.html \
  --cache-control "public, max-age=0, must-revalidate"
```

#### 5. Create CloudFront Distribution

```bash
aws cloudfront create-distribution \
  --origin-domain-name dreamup-qa-frontend.s3.us-east-1.amazonaws.com \
  --default-root-object index.html \
  --comment "DreamUp QA Agent Frontend"
```

#### 6. Configure Custom Error Pages

Set CloudFront error page for 404 to redirect to `/index.html` (for client-side routing).

### Option 2: Netlify

#### 1. Install Netlify CLI

```bash
npm install -g netlify-cli
```

#### 2. Update API URL and Build

Edit `frontend/src/Main.elm` with production API URL, then:
```bash
npm run build
```

#### 3. Deploy

```bash
cd dist
netlify deploy --prod
```

Create `netlify.toml` for automatic deployments:
```toml
[build]
  publish = "dist"
  command = "npm run build"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
```

### Option 3: Vercel

#### 1. Install Vercel CLI

```bash
npm install -g vercel
```

#### 2. Update API URL and Deploy

```bash
vercel --prod
```

Create `vercel.json`:
```json
{
  "buildCommand": "npm run build",
  "outputDirectory": "dist",
  "rewrites": [
    { "source": "/(.*)", "destination": "/index.html" }
  ]
}
```

## Environment-Specific Configuration

### Development
```elm
apiBaseUrl = "http://localhost:8080/api"
```

### Staging
```elm
apiBaseUrl = "https://staging-api.yourdomain.com"
```

### Production
```elm
apiBaseUrl = "https://api.yourdomain.com"
```

## Performance Optimizations

### 1. Enable Gzip/Brotli Compression

S3/CloudFront automatically serves `.gz` files if available:
```bash
# Vite already generates gzipped files during build
ls -lh dist/assets/*.js.gz
```

### 2. Set Cache Headers

- **Assets** (`/assets/*`): `max-age=31536000, immutable`
- **HTML** (`index.html`): `max-age=0, must-revalidate`

### 3. CloudFront Optimizations

- Enable HTTP/2 and HTTP/3
- Use Lambda@Edge for security headers
- Enable compression
- Set TTL: 1 year for assets, 0 for HTML

### 4. Content Security Policy

Add to CloudFront Lambda@Edge or Netlify headers:
```
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://api.yourdomain.com
```

## Monitoring and Analytics

### CloudWatch Metrics (for S3/CloudFront)
- Requests
- Bytes downloaded
- Error rates

### Google Analytics (Optional)

Add to `index.html`:
```html
<script async src="https://www.googletagmanager.com/gtag/js?id=GA_MEASUREMENT_ID"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'GA_MEASUREMENT_ID');
</script>
```

## Rollback Procedure

### S3/CloudFront

```bash
# List previous versions
aws s3api list-object-versions --bucket dreamup-qa-frontend

# Restore specific version
aws s3api copy-object \
  --copy-source "dreamup-qa-frontend/index.html?versionId=VERSION_ID" \
  --bucket dreamup-qa-frontend \
  --key index.html

# Invalidate CloudFront cache
aws cloudfront create-invalidation \
  --distribution-id DISTRIBUTION_ID \
  --paths "/*"
```

### Netlify/Vercel

Use the web dashboard to rollback to a previous deployment.

## Health Checks

Monitor:
- Frontend availability (GET /)
- API connectivity (test submissions)
- Asset loading times
- JavaScript errors (use Sentry or similar)

## Security Best Practices

1. **HTTPS Only**: Enforce HTTPS in CloudFront/Netlify/Vercel
2. **CORS**: Configure API CORS to allow only production domain
3. **CSP**: Add Content Security Policy headers
4. **SRI**: Consider Subresource Integrity for external scripts
5. **Rate Limiting**: Implement at API level

## Cost Estimates (AWS)

- **S3**: $0.023/GB storage + $0.09/GB data transfer
- **CloudFront**: $0.085/GB data transfer (first 10 TB)
- **Estimated**: ~$10-50/month for moderate traffic (10K users/month)

## Troubleshooting

### 404 Errors on Refresh

**Problem**: Direct navigation to `/report/123` returns 404
**Solution**: Configure rewrites to serve `index.html` for all routes

### CORS Errors

**Problem**: API requests blocked by CORS
**Solution**: Update API CORS to include production domain

### Slow Load Times

**Problem**: Assets loading slowly
**Solution**:
- Enable CloudFront compression
- Check asset sizes with `npm run build`
- Use CDN edge locations closer to users

## Additional Resources

- [Elm Deployment Guide](https://guide.elm-lang.org/optimization/asset_size.html)
- [AWS S3 Static Website Hosting](https://docs.aws.amazon.com/AmazonS3/latest/userguide/WebsiteHosting.html)
- [CloudFront Documentation](https://docs.aws.amazon.com/cloudfront/)
- [Netlify Deployment](https://docs.netlify.com/)
- [Vercel Deployment](https://vercel.com/docs)
