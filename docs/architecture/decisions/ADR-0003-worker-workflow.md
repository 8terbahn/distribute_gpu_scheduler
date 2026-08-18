# ADR-0003: Worker Automation & Fault Tolerance Design

## Status
Accepted

## Context
Previously, the Worker (written in Python) relied on a simple polling mechanism. If a temporary network anomaly occurred or the scheduler experienced a brief crash, the Worker would throw an exception and exit. Furthermore, due to the lack of a liveness detection mechanism, crashed Workers were still considered alive in the scheduling pool, leading to resource deadlocks.

## Decision
Enhance the automation capabilities and fault tolerance of the Python Worker:
1. **Heartbeat Mechanism**: A background thread in the Worker periodically sends heartbeats to the Control Plane to maintain the available resource quota of its `GPUPool`.
2. **Retry Strategy**: Implement an Exponential Backoff retry mechanism to handle transient network errors (e.g., 503, Timeout) when communicating with the Control Plane.
3. **Isolation and Decoupling**: Standardize `/worker/src` and `/worker/tests` so that automated testing can intercept HTTP requests for mock testing.

## Consequences
**Positive Impacts**:
- **Greatly Improved Resiliency**: The Worker can withstand brief control plane unavailability, achieving true distributed elasticity.
- **Optimized Resource Utilization**: Heartbeat-based liveness monitoring enables the control plane to quickly reclaim lost GPU capacity due to Worker Out-Of-Memory (OOM) or other failures.
- **Improved Code Quality**: Scheduling business logic is guaranteed through comprehensive unit tests.

**Negative Impacts**:
- Increased engineering complexity on the Python side.
