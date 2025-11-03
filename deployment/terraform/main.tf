terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# S3 bucket for QA artifacts
resource "aws_s3_bucket" "qa_artifacts" {
  bucket = var.s3_bucket_name

  tags = {
    Name        = "DreamUp QA Artifacts"
    Environment = var.environment
  }
}

# S3 bucket versioning
resource "aws_s3_bucket_versioning" "qa_artifacts" {
  bucket = aws_s3_bucket.qa_artifacts.id

  versioning_configuration {
    status = "Enabled"
  }
}

# IAM role for Lambda
resource "aws_iam_role" "lambda_role" {
  name = "${var.function_name}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# IAM policy for S3 access
resource "aws_iam_role_policy" "lambda_s3_policy" {
  name = "${var.function_name}-s3-policy"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.qa_artifacts.arn,
          "${aws_s3_bucket.qa_artifacts.arn}/*"
        ]
      }
    ]
  })
}

# Attach CloudWatch Logs policy
resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Lambda function
resource "aws_lambda_function" "qa_agent" {
  filename         = "../../lambda-deployment.zip"
  function_name    = var.function_name
  role            = aws_iam_role.lambda_role.arn
  handler         = "bootstrap"
  source_code_hash = filebase64sha256("../../lambda-deployment.zip")
  runtime         = "provided.al2"
  timeout         = 300
  memory_size     = 2048

  environment {
    variables = {
      S3_BUCKET_NAME   = aws_s3_bucket.qa_artifacts.id
      AWS_REGION       = var.aws_region
      OPENAI_API_KEY   = var.openai_api_key
    }
  }

  tags = {
    Name        = "DreamUp QA Agent"
    Environment = var.environment
  }
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = 7

  tags = {
    Environment = var.environment
  }
}

# Lambda function URL (optional - for HTTP invocation)
resource "aws_lambda_function_url" "qa_agent_url" {
  count = var.enable_function_url ? 1 : 0

  function_name      = aws_lambda_function.qa_agent.function_name
  authorization_type = "NONE" # Change to AWS_IAM for production

  cors {
    allow_origins = ["*"]
    allow_methods = ["POST"]
    max_age       = 86400
  }
}

# Outputs
output "lambda_function_arn" {
  description = "ARN of the Lambda function"
  value       = aws_lambda_function.qa_agent.arn
}

output "lambda_function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.qa_agent.function_name
}

output "s3_bucket_name" {
  description = "Name of the S3 artifacts bucket"
  value       = aws_s3_bucket.qa_artifacts.id
}

output "lambda_function_url" {
  description = "Lambda function URL (if enabled)"
  value       = var.enable_function_url ? aws_lambda_function_url.qa_agent_url[0].function_url : null
}
