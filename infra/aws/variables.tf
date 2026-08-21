variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "livestream-commerce"
}

variable "environment" {
  description = "Deployment environment tag (demo/staging/prod)"
  type        = string
  default     = "demo"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.43.0.0/16"
}

variable "eks_node_instance_types" {
  description = "Instance types for the EKS managed node group"
  type        = list(string)
  default     = ["t3.medium"]
}

variable "eks_node_desired_size" {
  description = "Desired EKS worker node count -- higher than fraud-detection's default since this platform runs 9 backend services + a frontend, each at 2+ replicas"
  type        = number
  default     = 4
}

variable "eks_node_min_size" {
  type    = number
  default = 3
}

variable "eks_node_max_size" {
  type    = number
  default = 10
}

variable "db_instance_class" {
  description = "RDS instance class -- small on purpose, this is a demo/portfolio deployment, not a sized-for-load one"
  type        = string
  default     = "db.t4g.micro"
}

# One RDS instance per service database, mirroring docker-compose.yml's
# one-Postgres-container-per-service pattern -- matches user == db name
# == service name, same as every *-postgres block in docker-compose.yml.
variable "postgres_services" {
  description = "Services that get their own RDS Postgres instance"
  type        = list(string)
  default     = ["auth", "stream", "chat", "subscription", "payment", "commerce", "notification"]
}

variable "db_password" {
  description = "Shared RDS master password across all per-service instances. Each service still gets its own instance/security group, so this only shortcuts secret-management for a demo deployment -- give each a distinct password (or a Secrets Manager-generated one, see rds.tf's manage_master_user_password note) before this is anything but a reference. Pass via TF_VAR_db_password -- never commit a real value."
  type        = string
  sensitive   = true
}

variable "redis_node_type" {
  description = "ElastiCache node type -- small on purpose, see db_instance_class"
  type        = string
  default     = "cache.t4g.micro"
}

# One ElastiCache instance per service that owns Redis as its system of
# record or cache, mirroring docker-compose.yml's *-redis containers.
# stream-redis exists locally too but only backs WebRTC signaling state
# that's fine to lose on a redeploy, so it isn't promoted to ElastiCache.
variable "redis_services" {
  description = "Services that get their own ElastiCache Redis instance"
  type        = list(string)
  default     = ["chat", "notification", "recommendation"]
}

variable "ecr_repositories" {
  description = "One ECR repo per service image, plus the frontend"
  type        = list(string)
  default = [
    "auth-service",
    "stream-service",
    "chat-service",
    "subscription-service",
    "payment-service",
    "commerce-service",
    "notification-service",
    "search-service",
    "recommendation-service",
    "livestream-frontend",
  ]
}
