output "cluster_name" {
  value = module.eks.cluster_name
}

output "configure_kubectl" {
  description = "Run this to point kubectl at the new cluster"
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.eks.cluster_name}"
}

output "rds_endpoints" {
  description = "Per-service Postgres endpoints -- goes into each service's DATABASE_URL"
  value       = { for service, db in module.rds : service => db.db_instance_endpoint }
  sensitive   = true
}

output "redis_endpoints" {
  description = "Per-service Redis endpoints -- goes into each service's REDIS_URL"
  value       = { for service, r in aws_elasticache_cluster.redis : service => r.cache_nodes[0].address }
}

output "ecr_repository_urls" {
  value = { for k, v in aws_ecr_repository.service : k => v.repository_url }
}
