# GPU Platform – Infrastructure Control Plane & Workflow Automation

> A cost-aware, Kubernetes-native workload orchestration engine. Built with Go and the Operator pattern, it enables product teams to self-serve GPU compute environments while automatically enforcing budget constraints, cost attribution, and compliance workflows.

**Part of the [GPU Compute Economics Platform](#related-projects)** — this is the decision layer. It consumes pricing data from [gpu_price_intelligence](https://github.com/8terbahn/gpu_price_intelligence) and performance baselines from [gpu_benchmark_platform](https://github.com/8terbahn/gpu_benchmark_platform) to optimally schedule workloads.

### Financial Compliance & Automation
This project demonstrates patterns commonly found in heavily regulated environments (like FinTech):
- **Approval Workflows:** Embedded JavaScript automation (via goja) enforces budget checks and management escalations before resource allocation, mirroring trade limit approvals.
- **Policy-as-Code:** Kyverno + CEL enforces strict cost-center tagging and hardware allow-lists at admission time.
- **Declarative Control Plane:** Crossplane abstracts underlying infrastructure into a unified self-service API.

---

## Architecture

```
Product Team
  │  kubectl apply -f gpu-env.yaml
  ▼
┌─────────────────────────────────────────────────────────────┐
│                 Kyverno Admission Policies                   │
│   (label validation, GPU type allow-list, unit limits)      │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│              Crossplane Composition Engine                    │
│   XGPUEnvironment XRD → Composition → function-gpu-sizing   │
│   Provisions: Namespace, ResourceQuota, NetworkPolicy,       │
│               GPUPool CR                                     │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│      Kubernetes Operator (controller-runtime / Kubebuilder)  │
│                                                              │
│  GPUWorkloadReconciler          GPUPoolReconciler            │
│  ┌───────────────────────┐      ┌──────────────────────┐    │
│  │ Phase state machine:  │      │ Recomputes capacity   │    │
│  │  Queued → Assigned    │      │ from live workload    │    │
│  │         → Running     │      │ scan (self-healing)   │    │
│  │         → Completed   │      └──────────────────────┘    │
│  │         → Failed      │                                   │
│  │  Finalizer: releases  │                                   │
│  │  pool capacity        │                                   │
│  └───────────────────────┘                                   │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│   Legacy Execution Layer (retained, backwards-compatible)    │
│   Go/Gin Scheduler REST API ←→ Redis ←→ PostgreSQL          │
│            ↕                                                 │
│   Python Worker Agents (heartbeat, GPU job simulation)       │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│         Workflow Automation Engine (Go + goja)               │
│   POST /workflows/approval_workflow/run                      │
│   Runs: workflow/scripts/approval_workflow.js               │
│    1. Compliance validation (cost center, GPU type)          │
│    2. Budget quota check (FinOps API)                        │
│    3. Auto-approve ≤4 units; escalate larger requests        │
│    4. Create K8s Claim + CMDB registration                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Technology Stack

| Layer | Technology | Purpose |
|---|---|---|
| **Control Plane** | Crossplane (XRD + Composition + Go Function) | Self-service GPU environment API |
| **Operator** | Kubebuilder + controller-runtime (Go) | GPUWorkload & GPUPool lifecycle |
| **Workflow** | Go + goja + JavaScript | Approval, quota, CMDB automation |
| **Policy** | Kyverno (ClusterPolicy + CEL) | Admission-time compliance |
| **Observability** | Prometheus (PrometheusRule) + Grafana | Alerting, dashboards, SLO tracking |
| **Packaging / CD** | Helm + GitHub Actions | 6-stage CI: lint → test → build → scan → chart → E2E |
| **Scheduling** | Go / Gin API, Redis, PostgreSQL | Priority-based GPU scheduling (original) |
| **Workers** | Python | Worker agents with heartbeat and GPU simulation |
| **Runtime** | Kubernetes (Kind locally, any K8s in production) | Container orchestration |

---

## Project Structure

```
distribute_gpu_scheduler/
│
├── operator/                        ← Kubernetes Operator (NEW – primary component)
│   ├── api/v1alpha1/
│   │   ├── groupversion_info.go     # API group: scheduling.gpu-platform.io
│   │   ├── gpuworkload_types.go     # GPUWorkload CRD spec / status
│   │   └── gpupool_types.go         # GPUPool CRD spec / status
│   ├── controllers/
│   │   ├── gpuworkload_controller.go # Reconcile: state machine + finalizer
│   │   ├── gpupool_controller.go    # Reconcile: capacity recomputation
│   │   └── suite_test.go            # envtest integration tests
│   ├── cmd/main.go                  # Operator entry point
│   ├── go.mod                       # Module: github.com/8terbahn/distribute_gpu_scheduler/operator
│   └── Dockerfile                   # Multi-stage: alpine builder → distroless nonroot
│
├── scheduler/                       ← Go Scheduler (clean rename of scheduler-go/)
│   ├── cmd/api/main.go
│   ├── internal/                    # (see scheduler-go/ for full source)
│   ├── go.mod
│   └── Dockerfile
│
├── worker/                          ← Python Workers (clean rename of worker-python/)
│   ├── worker/
│   ├── requirements.txt
│   └── Dockerfile
│
├── crossplane/                      ← Crossplane Control Plane
│   ├── xrd.yaml                     # XGPUEnvironment XRD
│   ├── composition.yaml             # Namespace + Quota + NetworkPolicy + GPUPool
│   ├── provider-config.yaml         # Kubernetes ProviderConfig (workload identity)
│   └── functions/gpu-sizing/
│       ├── main.go                  # Go gRPC Crossplane Function
│       └── go.mod
│
├── workflow/                        ← Automation
│   ├── scripts/
│   │   └── approval_workflow.js     # JS: compliance, budget, approval, K8s, CMDB
│   └── engine/
│       ├── engine.go                # Go HTTP server executing JS via goja
│       └── Dockerfile
│
├── policies/kyverno/                ← Policy-as-Code
│   ├── gpu-policies.yaml            # Label, GPU type, unit limit, Pod security
│   └── validation-rules.yaml        # CEL: cost-center format, priority, expiry
│
├── monitoring/                      ← Observability
│   ├── prometheus-rules.yaml        # PrometheusRule CRD (alerts + recording rules)
│   └── grafana-dashboard.json       # Importable Grafana dashboard
│
├── charts/gpu-platform/             ← Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml          # Operator + Workflow Engine
│       └── rbac.yaml                # ServiceAccount, ClusterRole, ClusterRoleBinding
│
├── config/crd/bases/                ← CRD Manifests (apply before Helm)
│   ├── scheduling.gpu-platform.io_gpuworkloads.yaml
│   └── scheduling.gpu-platform.io_gpupools.yaml
│
├── docs/                            ← Documentation
│   ├── ADR.md                       ← Consolidated architecture decisions
│   ├── developer-guide.md           # Self-service guide for product teams
│   └── troubleshooting-runbook.md   # SRE incident response (9 scenarios)
│
├── .github/workflows/ci.yml         ← CI/CD (lint→test→build→scan→chart→E2E)
├── docker-compose.yml               ← Local dev (Redis + Postgres)
└── README.md

# Legacy directories (retained for backwards compatibility)
├── scheduler-go/                    ← Original Go scheduler source
└── worker-python/                   ← Original Python worker source
```

---

## Quick Start

### Option A – Local Development (no cluster required)

```bash
# Start dependencies
docker compose up -d

# Run the legacy Go scheduler
cd scheduler-go && go run ./cmd/api

# Run a Python worker
cd worker-python && python -m worker.main

# Submit a test job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"workload":"llm_inference","gpu_requirement":"A100","priority":7}'
```

### Option B – Kubernetes Operator (Kind cluster)

```bash
# 1. Create cluster
kind create cluster --name gpu-platform

# 2. Install CRDs
kubectl apply -f config/crd/bases/

# 3. Install Kyverno (policy engine)
helm repo add kyverno https://kyverno.github.io/kyverno/
helm install kyverno kyverno/kyverno -n kyverno --create-namespace --wait

# 4. Deploy the platform
helm install gpu-platform charts/gpu-platform \
  --namespace gpu-platform --create-namespace

# 5. Apply policies + Crossplane manifests
kubectl apply -f policies/kyverno/
kubectl apply -f crossplane/   # requires Crossplane installed

# 6. Create a GPU Pool
kubectl apply -f - <<EOF
apiVersion: scheduling.gpu-platform.io/v1alpha1
kind: GPUPool
metadata:
  name: a100-pool
  namespace: gpu-platform
  labels:
    gpu-platform.io/cost-center: CC-0001
    gpu-platform.io/owner: platform-team
spec:
  gpuType: A100
  totalUnits: 8
EOF

# 7. Submit a GPU Workload
kubectl apply -f - <<EOF
apiVersion: scheduling.gpu-platform.io/v1alpha1
kind: GPUWorkload
metadata:
  name: inference-job-1
  namespace: gpu-platform
  labels:
    gpu-platform.io/cost-center: CC-0001
    gpu-platform.io/owner: ml-team
spec:
  workload: llm_inference
  gpuRequirement: A100
  priority: 8
EOF

# 8. Watch the scheduling lifecycle
kubectl get gpuworkloads -n gpu-platform -w
```

### Option C – Self-Service (product team perspective)

See [docs/developer-guide.md](docs/developer-guide.md).

---

## CI/CD Pipeline

`.github/workflows/ci.yml` runs on every push:

| Stage | What it does |
|---|---|
| **Lint** | `golangci-lint` (Go) + ESLint (JavaScript) |
| **Test** | `envtest` integration tests + JS workflow tests + 70% coverage gate |
| **Build** | Docker multi-stage: operator, workflow engine, Crossplane function |
| **Scan** | Trivy (CRITICAL/HIGH blocks merge) + SARIF upload to GitHub Security |
| **Chart** | `helm lint` + template render + OCI push to GHCR |
| **E2E** | Kind cluster: CRDs, chart deploy, GPUWorkload submit, assert `Assigned` phase |

---

## Observability

```bash
# Metrics (Prometheus)
kubectl port-forward svc/gpu-platform-operator-metrics 8081:8081 -n gpu-platform
curl http://localhost:8081/metrics | grep gpu_platform

# Grafana dashboard
# Dashboards → Import → Upload monitoring/grafana-dashboard.json
```

Key metrics:

| Metric | Description |
|---|---|
| `gpu_platform_workloads_total{status}` | Workload throughput by phase |
| `gpu_platform_pool_available_units` | Real-time pool capacity |
| `controller_runtime_reconcile_time_seconds` | Operator reconcile latency histogram |
| `gpu_platform_queue_depth` | Scheduling backlog |

---

## Documentation

| Document | Purpose |
|---|---|
| [ADR.md](docs/ADR.md) | All architecture decisions (Crossplane, Operator, JS engine) |
| [developer-guide.md](docs/developer-guide.md) | Self-service onboarding for product teams |
| [troubleshooting-runbook.md](docs/troubleshooting-runbook.md) | SRE incident response for 9 failure scenarios |

---

## Design Highlights

- **Declarative self-healing**: `GPUWorkload` CRs are level-triggered; the operator
  self-heals from crashes, pod restarts, and network partitions.
- **Finalizer-based cleanup**: pool capacity is released even on forceful deletion —
  no resource leaks.
- **Compliance-by-default**: Kyverno enforces cost-center tagging, GPU type allow-lists,
  and Pod security at admission time — before any resource reaches etcd.
- **Multi-language workflow automation**: a Go HTTP server executes JavaScript business
  logic via goja — no Node.js runtime required in production.
- **GitOps-ready**: Helm chart + GitHub Actions CI → ArgoCD → reproducible, auditable
  deployments.
- **SRE-grade observability**: `PrometheusRule` CRD alerts + pre-built Grafana dashboard
  + 9-scenario troubleshooting runbook.


---

## Related Projects

This project is part of a three-tier platform for GPU compute economics:

| Layer | Project | Role |
|---|---|---|
| **Data Ingestion** | [gpu_price_intelligence](https://github.com/8terbahn/gpu_price_intelligence) | Collects and serves GPU pricing data |
| **Performance Evaluation** | [gpu_benchmark_platform](https://github.com/8terbahn/gpu_benchmark_platform) | Benchmarks GPU performance via K8s Operator |
| **Smart Scheduling** | **distribute_gpu_scheduler** (this repo) | Cost-optimized workload orchestration |

### Financial Domain Parallels

The architecture of this scheduling system mirrors patterns found in financial trading platforms:

| This Platform | Financial Equivalent |
|---|---|
| Budget limit validation before GPU scheduling | Pre-trade risk limit checks |
| Cost-optimized workload placement | Best execution / smart order routing |
| Kyverno admission policies | Regulatory rule engines (FINMA, MiFID II) |
| Multi-stage JS approval workflows | Multi-tier trade or loan approval systems |
| Prometheus SLO monitoring | Market connectivity monitoring |
