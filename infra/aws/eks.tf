# EKS cluster the existing infra/k8s/*.yaml manifests apply onto directly --
# this module only provisions the cluster + node group; the platform's
# workloads themselves stay as plain manifests (kubectl apply -f
# infra/k8s/), not re-expressed in Terraform, so the local Docker Compose
# and EKS paths run the exact same YAML.

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = "1.29"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access = true

  eks_managed_node_groups = {
    default = {
      instance_types = var.eks_node_instance_types
      desired_size   = var.eks_node_desired_size
      min_size       = var.eks_node_min_size
      max_size       = var.eks_node_max_size
    }
  }

  cluster_addons = {
    coredns    = {}
    kube-proxy = {}
    vpc-cni    = {}
  }

  # infra/k8s/15-ingress.yaml assumes the AWS Load Balancer Controller is
  # already running in-cluster to satisfy `kubernetes.io/ingress.class:
  # alb` -- it's a Helm chart + IRSA role, not an EKS managed add-on, so it
  # isn't provisioned by this module. Install it after `terraform apply`:
  # https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/deploy/installation/
  enable_irsa = true

  tags = {
    Environment = var.environment
  }
}
