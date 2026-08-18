# ADR-0002: Standard Project Layout Refactoring

## Status
Accepted

## Context
The previous project contained multiple independent submodules (like separate `operator/` and `scheduler/` folders). While this achieved some level of isolation, it led to:
1. Blurred module boundaries, with internal code dependencies (such as shared CRD structs) having potential cyclic references.
2. Non-conformance with the community-recommended [golang-standards/project-layout](https://github.com/golang-standards/project-layout) standard, increasing the onboarding cost for new members.
3. Multiple duplicated `go.mod` files that complicated version releases and Docker image builds.

## Decision
Consolidate the entire codebase into a single module root structure:
- **/cmd/scheduler**: Unified main entry point for the Kubernetes Operator / Scheduler.
- **/api/v1**: Contains all Kubernetes Custom Resource Definition (CRD) types (stripped from the previous `v1alpha1`), acting as the exposed API contract layer.
- **/internal/controllers**: Core controller directory containing the primary Reconcile loops for `GPUWorkload` and `GPUPool` (hidden from external packages to guarantee encapsulation).
- **/worker**: Organized the original Python Worker into explicit `/src` and `/tests` directories.

## Consequences
**Positive Impacts**:
- Clear structure that conforms to mainstream Go open-source community standards, greatly reducing onboarding and development costs.
- A single `go.mod` minimizes dependency fragmentation and conflicts.
- Centralizes CI/CD build pipelines.

**Negative Impacts**:
- Lost strong physical isolation; if there is a future desire to extract the scheduler into an independent component, `/internal` would need refactoring.
