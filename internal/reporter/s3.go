package reporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dreamup/qa-agent/internal/agent"
)

// S3Uploader handles uploading artifacts to S3
type S3Uploader struct {
	client     *s3.Client
	bucketName string
	region     string
}

// NewS3Uploader creates a new S3 uploader
func NewS3Uploader(bucketName, region string) (*S3Uploader, error) {
	if bucketName == "" {
		bucketName = os.Getenv("S3_BUCKET_NAME")
		if bucketName == "" {
			bucketName = "dreamup-qa-artifacts" // Default from PRD
		}
	}

	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1" // Default
		}
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Uploader{
		client:     client,
		bucketName: bucketName,
		region:     region,
	}, nil
}

// UploadFile uploads a file to S3
func (u *S3Uploader) UploadFile(ctx context.Context, filepath, s3Key string) (string, error) {
	// Read file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	// Determine content type
	contentType := u.getContentType(filepath)

	// Upload to S3
	_, err = u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucketName),
		Key:         aws.String(s3Key),
		Body:        strings.NewReader(string(data)),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Construct S3 URL
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		u.bucketName,
		u.region,
		s3Key,
	)

	return s3URL, nil
}

// getContentType determines content type from file extension
func (u *S3Uploader) getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// UploadScreenshot uploads a screenshot to S3
func (u *S3Uploader) UploadScreenshot(ctx context.Context, screenshot *agent.Screenshot, reportID string) (string, error) {
	// Generate S3 key
	s3Key := fmt.Sprintf("reports/%s/screenshots/%s_%s.png",
		reportID,
		screenshot.Context,
		screenshot.Timestamp.Format("20060102_150405"),
	)

	return u.UploadFile(ctx, screenshot.Filepath, s3Key)
}

// UploadReport uploads a report JSON to S3
func (u *S3Uploader) UploadReport(ctx context.Context, reportPath, reportID string) (string, error) {
	s3Key := fmt.Sprintf("reports/%s/report.json", reportID)
	return u.UploadFile(ctx, reportPath, s3Key)
}

// UploadConsoleLogs uploads console logs to S3
func (u *S3Uploader) UploadConsoleLogs(ctx context.Context, logPath, reportID string) (string, error) {
	s3Key := fmt.Sprintf("reports/%s/console_logs.json", reportID)
	return u.UploadFile(ctx, logPath, s3Key)
}

// UploadReportWithArtifacts uploads a complete report with all artifacts
func (u *S3Uploader) UploadReportWithArtifacts(ctx context.Context, report *Report, screenshots []*agent.Screenshot, logPath string) error {
	// Upload screenshots and update report
	for i, screenshot := range screenshots {
		s3URL, err := u.UploadScreenshot(ctx, screenshot, report.ReportID)
		if err != nil {
			return fmt.Errorf("failed to upload screenshot %d: %w", i, err)
		}
		// Update S3 URL in report
		if i < len(report.Evidence.Screenshots) {
			report.Evidence.Screenshots[i].S3URL = s3URL
		}
	}

	// Save updated report to temp file
	reportPath, err := report.SaveToTemp()
	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}
	defer os.Remove(reportPath)

	// Upload report
	_, err = u.UploadReport(ctx, reportPath, report.ReportID)
	if err != nil {
		return fmt.Errorf("failed to upload report: %w", err)
	}

	// Upload console logs if provided
	if logPath != "" {
		_, err = u.UploadConsoleLogs(ctx, logPath, report.ReportID)
		if err != nil {
			return fmt.Errorf("failed to upload console logs: %w", err)
		}
	}

	return nil
}

// GetReportURL returns the S3 URL for a report
func (u *S3Uploader) GetReportURL(reportID string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/reports/%s/report.json",
		u.bucketName,
		u.region,
		reportID,
	)
}
