# One RDS instance per service, replacing the matching in-cluster Postgres
# StatefulSet (infra/k8s/{05-auth,06-stream,07-chat,09-subscription,
# 08-payment,10-commerce,11-notification}.yaml) for production -- same
# db/user naming as docker-compose.yml so swapping each service's
# DATABASE_URL ConfigMap/env value is the only change needed on the app
# side, not a schema or connection-string-shape change.

resource "aws_security_group" "rds" {
  name_prefix = "${var.cluster_name}-rds-"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Postgres from EKS nodes"
    from_port       = 5432
    to_port         = 5432
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

resource "aws_db_subnet_group" "postgres" {
  name       = "${var.cluster_name}-postgres"
  subnet_ids = module.vpc.private_subnets
}

module "rds" {
  source   = "terraform-aws-modules/rds/aws"
  version  = "~> 6.0"
  for_each = toset(var.postgres_services)

  identifier = "${var.cluster_name}-${each.value}"

  engine               = "postgres"
  engine_version       = "16"
  family               = "postgres16"
  major_engine_version = "16"
  instance_class       = var.db_instance_class

  allocated_storage     = 20
  max_allocated_storage = 100
  storage_encrypted     = true

  db_name  = each.value
  username = each.value
  password = var.db_password
  port     = 5432

  manage_master_user_password = false # explicit password var above; flip to true + drop var for Secrets Manager-managed rotation, ideally per-service

  vpc_security_group_ids = [aws_security_group.rds.id]
  create_db_subnet_group = false
  db_subnet_group_name   = aws_db_subnet_group.postgres.name

  multi_az            = false # single-AZ for a demo deployment; set true for production
  deletion_protection = false # false for a portfolio/demo project so `terraform destroy` actually tears it down
  skip_final_snapshot = true

  tags = { Environment = var.environment, Service = each.value }
}
