# ADR-0001: Evolving to a Kubernetes Operator Control Plane Pattern

## Status
Accepted

## Context
As the number of managed GPU nodes and tasks on the platform has surged, the original standalone scheduler based on a REST API (HTTP + Redis + Postgres) exposed critical issues:
1. **State Loss**: If the scheduler crashes or restarts, running tasks lose their tracking context, and there is no native self-healing capability.
2. **Imperative API**: Users can only submit tasks via HTTP POST, lacking the ability to use `kubectl apply` for version control and GitOps management like native Kubernetes resources.
3. **Poor Scalability**: Multi-replica scheduling easily causes split-brain problems and requires complex distributed locking mechanisms.

## Decision
Refactor the core GPU scheduling and allocation logic entirely into a Kubernetes Operator pattern (using the Controller paradigm):
- Deprecate the original HTTP REST API in favor of watching two Custom Resources (CRDs) in Kubernetes: `GPUWorkload` and `GPUPool`.
- Utilize the `controller-runtime` framework to implement a highly available Controller with Leader Election.
- Perform all scheduling actions (matching idle GPUs, binding workers) within the Reconcile loop, using Kubernetes Etcd as the Single Source of Truth.

## Consequences
**Positive Impacts**:
- **Declarative Management**: Data scientists can submit `GPUWorkload` just like defining Pods, seamlessly integrating with existing cloud-native pipelines.
- **Self-Healing Capability**: Thanks to the level-triggered Reconcile mechanism, any state drift or failure will trigger retries until the desired state is met.
- **High Availability**: Native distributed lock support allows running multiple replicas seamlessly.

**Negative Impacts**:
- Increased development threshold, requiring the team to master Kubernetes API mechanisms and the Controller programming model.
- No longer suitable for standalone operation outside a Kubernetes environment.
