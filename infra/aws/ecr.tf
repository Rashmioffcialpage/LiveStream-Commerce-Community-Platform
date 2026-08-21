# One repo per service image, plus the frontend -- push
# services/*/Dockerfile and frontend/Dockerfile builds here instead of a
# placeholder Docker Hub namespace before applying infra/k8s/*.yaml against
# a real EKS cluster (the manifests currently reference
# rashmioffcialpage/*:latest; repoint each image field at
# ${aws_ecr_repository.service["<name>"].repository_url}:<tag>).

resource "aws_ecr_repository" "service" {
  for_each = toset(var.ecr_repositories)

  name                 = "${var.cluster_name}/${each.value}"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = { Environment = var.environment }
}

resource "aws_ecr_lifecycle_policy" "service" {
  for_each = aws_ecr_repository.service

  repository = each.value.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}
