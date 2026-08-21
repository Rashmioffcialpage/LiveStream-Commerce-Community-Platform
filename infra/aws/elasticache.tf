# One ElastiCache instance per service, replacing the matching in-cluster
# Redis StatefulSet (infra/k8s/{07-chat,11-notification,13-recommendation}.
# yaml) for production. Single node each -- chat-service uses Redis for
# Pub/Sub fan-out, notification-service for the same, recommendation-
# service for its affinity sorted sets as the system of record itself, none
# of which need cross-node clustering at demo scale.

resource "aws_security_group" "redis" {
  name_prefix = "${var.cluster_name}-redis-"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Redis from EKS nodes"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Environment = var.environment }
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.cluster_name}-redis"
  subnet_ids = module.vpc.private_subnets
}

resource "aws_elasticache_cluster" "redis" {
  for_each = toset(var.redis_services)

  cluster_id           = "${var.cluster_name}-${each.value}"
  engine               = "redis"
  engine_version       = "7.1"
  node_type            = var.redis_node_type
  num_cache_nodes      = 1
  port                 = 6379
  parameter_group_name = "default.redis7"

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.redis.id]

  tags = { Environment = var.environment, Service = each.value }
}
