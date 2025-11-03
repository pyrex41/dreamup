variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "function_name" {
  description = "Lambda function name"
  type        = string
  default     = "dreamup-qa-agent"
}

variable "s3_bucket_name" {
  description = "S3 bucket name for QA artifacts"
  type        = string
  default     = "dreamup-qa-artifacts"
}

variable "openai_api_key" {
  description = "OpenAI API key for LLM evaluation (optional - can be set directly in AWS Secrets Manager)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "enable_function_url" {
  description = "Enable Lambda function URL for HTTP access"
  type        = bool
  default     = false
}
