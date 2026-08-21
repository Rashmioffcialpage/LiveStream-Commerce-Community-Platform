terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Uncomment and point at a real bucket before applying for real -- local
  # state is fine for a first `plan`, not for a team or CI.
  # backend "s3" {
  #   bucket = "your-tfstate-bucket"
  #   key    = "livestream-commerce-platform/terraform.tfstate"
  #   region = "us-east-1"
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = "livestream-commerce-platform"
      ManagedBy = "terraform"
    }
  }
}
