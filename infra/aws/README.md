# AWS deployment (Terraform)

Provisions the managed AWS services `infra/k8s/`'s per-file comments
recommend in place of the demo-grade in-cluster ones:

| Resource | Replaces |
|---|---|
| EKS cluster + managed node group | — (the cluster itself) |
| 7× RDS PostgreSQL (one per service) | each service's own `*-postgres` StatefulSet |
| 3× ElastiCache Redis (chat, notification, recommendation) | each service's own `*-redis` StatefulSet |
| 10× ECR repository | the placeholder `rashmioffcialpage/*:latest` images (9 services + the frontend) |

Kafka, OpenSearch, and MinIO are deliberately **not** provisioned here --
they stay self-hosted in-cluster (`infra/k8s/02-kafka.yaml`,
`03-opensearch.yaml`, `04-minio.yaml`) even on EKS. Real upgrade paths if
that changes: Amazon MSK for Kafka, Amazon OpenSearch Service for search,
real S3 for MinIO (stream-service's S3 client already points at a
configurable endpoint -- see `docs/task-recordings.md`).

This directory provisions infrastructure only. The application workloads
stay exactly the plain manifests in `infra/k8s/` -- apply them with
`kubectl` after this Terraform run, so the same YAML works unmodified
against both local Docker Compose's service topology and this EKS
cluster.

## ⚠️ Before you run this

- **This costs real money** -- NAT gateway, EKS control plane, 7 RDS
  instances, and 3 ElastiCache instances are all billed hourly even at the
  smallest sizes here. `terraform destroy` when you're done evaluating it.
- Nothing in this repo runs `terraform apply` for you. Review every
  resource below and the plan output first.
- Requires an AWS account, credentials configured (`aws configure` /
  `AWS_PROFILE`), and Terraform ≥ 1.5.

## Usage

```bash
cd infra/aws
terraform init

# never pass a real password as a CLI arg (shell history); export it instead
export TF_VAR_db_password='replace-with-a-generated-secret'

terraform plan
terraform apply

# point kubectl at the new cluster
$(terraform output -raw configure_kubectl)

# install the AWS Load Balancer Controller before applying the Ingress --
# it's Helm + IRSA, not something this module provisions (see eks.tf)
# https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/deploy/installation/

# apply the application manifests (same YAML as the local docs)
kubectl apply -f ../k8s/00-namespace.yaml
kubectl apply -f ../k8s/
```

Before applying the app manifests for real:

- Update each service's `DATABASE_URL`/`REDIS_URL` in `infra/k8s/*.yaml`
  to the matching `terraform output rds_endpoints` / `redis_endpoints`
  value (both are keyed by service name).
- Push each `services/*/Dockerfile` and `frontend/Dockerfile` build to the
  matching repo in `terraform output ecr_repository_urls` instead of the
  placeholder `rashmioffcialpage/*:latest` images -- for the frontend,
  rebuild with `NEXT_PUBLIC_*_URL` build args pointing at the real hosts
  from `infra/k8s/15-ingress.yaml` first (see `frontend/Dockerfile`'s
  top comment for why those can't be a runtime env var).
- Point DNS for every host in `infra/k8s/15-ingress.yaml` at the ALB the
  AWS Load Balancer Controller provisions once the Ingress is applied.

## Teardown

```bash
terraform destroy
```
