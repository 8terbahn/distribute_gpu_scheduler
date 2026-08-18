---
# Architecture Decision Record 0001
# Status: Accepted

# ADR-0001: Adopt Crossplane as the Infrastructure Control Plane

## Status
**Accepted** – 2026-08-18

## Context and Problem Statement

The 8terbahn GPU Platform team needed to provide product teams with a reliable,
self-service way to provision GPU compute environments. Historically, this required
manual tickets and Ops intervention, resulting in:

- Multi-day lead times for environment setup.
- Inconsistent resource tagging and cost-center attribution.
- No audit trail or declarative state.
- Difficulty enforcing compliance policies (GPU type allow-lists, quotas).

We evaluated three approaches:

| Option | Description | Key Trade-off |
|---|---|---|
| **A: Manual scripts + ITSM tickets** | Ops team executes Bash/Ansible on request | Fast to start, does not scale, no self-service |
| **B: Custom REST API (current state)** | Go/Gin API wrapping Redis + Postgres | Good control, requires building reconciliation from scratch |
| **C: Crossplane + Kubernetes Operator** | Declarative CRDs, Crossplane XRD/Compositions | Steeper learning curve, but industry-standard self-service pattern |

## Decision

We adopt **Option C: Crossplane + Kubernetes Operator**.

Concretely:
- A **Kubebuilder Operator** (`GPUWorkload`, `GPUPool` CRDs) handles scheduling and
  pool lifecycle within the cluster.
- A **Crossplane Composition** (`XGPUEnvironment`) exposes a higher-level self-service
  API to product teams. A Go-based **Crossplane Function** (`function-gpu-sizing`)
  handles dynamic sizing logic and compliance annotation injection.
- The existing Go scheduler and Python workers are retained as the execution backend,
  now managed by the Operator instead of a standalone HTTP API.

## Rationale

1. **Declarative state**: Kubernetes reconciliation guarantees that desired state is
   continuously enforced, even after failures or node restarts.
2. **Self-service without tickets**: Teams can `kubectl apply -f my-gpu-env.yaml`
   and the Crossplane Composition handles everything downstream.
3. **Compliance-by-default**: Kyverno admission policies are enforced at the API
   server level before any resource is created – no policy can be bypassed.
4. **Portability**: Crossplane Compositions abstract the underlying cloud provider.
   Migrating from on-prem to AWS/GCP requires only a new ProviderConfig and updated
   Composition; the team-facing API (`GPUEnvironment`) stays identical.
5. **Industry alignment**: Crossplane is a CNCF incubating project widely adopted in
   enterprise platform engineering (Julius Baer peers in finance and beyond).
6. **Operational observability**: controller-runtime exposes standard Prometheus metrics
   (`controller_runtime_reconcile_*`) out of the box.

## Consequences

### Positive
- Product teams can provision GPU environments in < 2 minutes without Ops involvement.
- All resources are label-enforced and cost-center-attributed at admission time.
- The platform is composable: new GPU types or cloud backends require only a new
  Composition, not code changes to the operator.

### Negative / Risks
- Learning curve for engineers unfamiliar with Crossplane (mitigated by documentation
  and workshops).
- Crossplane adds operational complexity (additional control-plane components to manage).
- Composition debugging can be opaque; mitigation: thorough tracing via `crossplane beta trace`.

## References
- [Crossplane Docs](https://docs.crossplane.io)
- [Kubebuilder Book](https://book.kubebuilder.io)
- [CNCF Platform Engineering Whitepaper](https://tag-app-delivery.cncf.io/whitepapers/platforms/)
