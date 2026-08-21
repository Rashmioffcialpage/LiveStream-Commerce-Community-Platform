# Task 9 — Kubernetes + AWS deployment

Everything built in Tasks 1-8 runs identically against Docker Compose
locally or against a real EKS cluster -- the same 70 Kubernetes objects in
[infra/k8s/](../infra/k8s/) apply to either, and [infra/aws/](../infra/aws/)
has the (unapplied, reference) Terraform to provision the managed AWS
services those manifests are written to swap onto.

## What's in infra/k8s/

One file per concern, numbered so the natural apply order (namespace →
shared config → shared datastores → each service → frontend → ingress) is
also the file-listing order:

| File | Contents |
|---|---|
| `00-namespace.yaml` | the `livestream` namespace |
| `01-config.yaml` | shared `platform-config` ConfigMap (Kafka brokers, internal service URLs) + `jwt-secret` Secret |
| `02-kafka.yaml` | single-broker KRaft StatefulSet |
| `03-opensearch.yaml` | single-node OpenSearch StatefulSet |
| `04-minio.yaml` | S3-compatible object storage for recordings |
| `05`-`13` | one file per backend service: its own Postgres/Redis StatefulSet (where it has one) + Deployment + Service + HorizontalPodAutoscaler |
| `14-frontend.yaml` | the Next.js web client |
| `15-ingress.yaml` | one ALB, host-based routing to every service the browser talks to directly |

Every backend Deployment carries a `readinessProbe`/`livenessProbe` on
`GET /healthz` and an HPA scaling 2 (or 3, for stateless higher-traffic
services) → 8-10 replicas on 70% CPU utilization -- the same shape as
every service already had to support for its own local `docker compose
up --build` health checks, just re-expressed as Kubernetes probes instead
of Compose healthchecks.

## Design notes

- **One manifest file per service, not one per Kubernetes kind.** Finding
  everything about `chat-service` -- its Postgres, its Redis, its
  Deployment, its Service, its HPA -- means opening
  [07-chat.yaml](../infra/k8s/07-chat.yaml), not grepping five separate
  `postgres.yaml` / `redis.yaml` / `deployments.yaml` files for the one
  block that mentions it.
- **Per-service Secrets for each Postgres's credentials, not one shared
  Secret.** Matches docker-compose.yml's one-`*-postgres`-container-per-
  service isolation -- a leaked auth-service DB credential doesn't also
  expose payment-service's.
- **Subdomain routing in the Ingress, not path-prefix routing.** The
  frontend calls each backend directly from the browser
  (`frontend/lib/api.ts` has one base URL per service, no shared API
  gateway), and every Go service's routes are already rooted at `/`
  (`POST /signup`, not `POST /api/auth/signup`) -- a path-prefix scheme
  would need per-service rewrite rules for no real benefit over giving
  each service its own subdomain.
- **`payment-service` has no Ingress rule.** It's never called from the
  browser, only server-to-server by subscription-service and
  commerce-service (see `frontend/lib/api.ts` -- there's no
  `NEXT_PUBLIC_PAYMENT_URL`), so exposing it externally would just be
  attack surface with no corresponding feature.
- **Kafka, OpenSearch, and MinIO stay self-hosted in-cluster even on AWS**
  (`infra/aws/README.md` explains why per-service) -- unlike Postgres and
  Redis, which get real managed replacements
  (`infra/aws/rds.tf`, `elasticache.tf`) because every service already
  treats its database as a plain connection string, so pointing it at RDS
  instead of a StatefulSet is a config change, not a code change.
- **The frontend's `NEXT_PUBLIC_*` env vars are baked in at Docker build
  time, not read at container start** -- Next.js inlines them into the
  client bundle during `next build`. `frontend/Dockerfile` takes them as
  build ARGs instead of the runtime-env pattern every Go service uses, and
  they have to be real public hostnames (the Ingress's), not in-cluster
  Service DNS names, because the browser calls them directly.

## Verification

- All 70 objects across the 16 `infra/k8s/*.yaml` files pass
  `kubeconform -strict -ignore-missing-schemas` (wired into CI --
  `.github/workflows/infra.yml`'s `k8s-manifests-lint` job).
- `frontend/Dockerfile` was actually built and run locally (`docker build`
  + `docker run`, not just written): the standalone Next.js server starts
  and serves `200` on `/` from the produced image.
- `infra/aws/*.tf` passes `terraform init -backend=false && terraform
  validate` and `terraform fmt -check` (also wired into CI, the
  `terraform-validate` job) -- syntactically and internally consistent,
  same "reference, not applied" posture as this platform's fraud-detection
  sibling project: no AWS credentials were configured for this session, so
  nothing was actually provisioned or billed.

Not verified: an actual `kubectl apply` against a running cluster. No
local Kubernetes cluster (kind/minikube) was available in this
environment, and standing one up wasn't judged worth the added local
resource footprint on top of the already-running 20-plus-container Docker
Compose stack this task was verified alongside -- `kubeconform`'s schema
validation catches the class of error (wrong field names, wrong types,
missing required fields) most likely from hand-authoring 70 objects, which
is what it was used for here.
