/**
 * approval_workflow.js
 *
 * Enterprise GPU Environment Approval Workflow
 *
 * This script implements a multi-step approval and provisioning workflow for
 * GPU compute environments. It is written to be engine-agnostic: the business
 * logic is pure JavaScript and can run inside Argo Workflows, n8n, Temporal,
 * or the project's own Go workflow engine (workflow/engine/engine.go).
 *
 * Workflow steps:
 *  1. Validate the incoming GPUEnvironment claim against compliance rules.
 *  2. Check the requester's budget quota via the FinOps API.
 *  3. Auto-approve small requests; escalate larger ones via a notification.
 *  4. Create the XGPUEnvironment claim in Kubernetes.
 *  5. Register the environment in the CMDB.
 *
 * Configuration is injected via environment variables (see CONFIG below).
 * Run locally for testing: node approval_workflow.js
 */
"use strict";

// ── Configuration ─────────────────────────────────────────────────────────────
const CONFIG = {
  FINOPS_API_URL:  process.env.FINOPS_API_URL  || "https://finops.example.internal",
  NOTIFY_WEBHOOK:  process.env.NOTIFY_WEBHOOK  || "",   // Slack / Teams / generic webhook
  K8S_API_URL:     process.env.K8S_API_URL     || "https://kubernetes.default.svc",
  CMDB_API_URL:    process.env.CMDB_API_URL    || "https://cmdb.example.internal",
  APPROVAL_TIMEOUT_MS:         60 * 60 * 1000, // 1 hour
  MAX_UNITS_WITHOUT_APPROVAL:  4,
  ALLOWED_GPU_TYPES:           ["T4", "V100", "A100", "H100"],
};

// ── Compliance Validation ─────────────────────────────────────────────────────
/**
 * Validates a GPUEnvironment claim against platform compliance rules.
 * @param {Object} claim
 * @returns {{ valid: boolean, errors: string[] }}
 */
function validateClaim(claim) {
  const errors = [];

  if (!claim.spec.costCenter || !/^CC-\d{4}$/.test(claim.spec.costCenter)) {
    errors.push("spec.costCenter must match format CC-NNNN (e.g. CC-4210).");
  }
  if (!claim.spec.owner || claim.spec.owner.trim() === "") {
    errors.push("spec.owner is required and must be non-empty.");
  }
  if (!CONFIG.ALLOWED_GPU_TYPES.includes(claim.spec.gpuType)) {
    errors.push(
      `spec.gpuType '${claim.spec.gpuType}' is not in the allow-list: ${CONFIG.ALLOWED_GPU_TYPES.join(", ")}.`
    );
  }
  if (typeof claim.spec.units !== "number" || claim.spec.units < 1) {
    errors.push("spec.units must be a positive integer.");
  }
  if (claim.spec.autoExpireHours && claim.spec.autoExpireHours > 168) {
    errors.push("spec.autoExpireHours must not exceed 168 (7 days).");
  }

  return { valid: errors.length === 0, errors };
}

// ── Budget Quota Check ────────────────────────────────────────────────────────
/**
 * Checks whether the cost center has remaining budget for this request.
 * In production, replace the mock with an actual HTTP call to your FinOps API.
 * @param {string} costCenter
 * @param {string} gpuType
 * @param {number} units
 * @param {number} hours
 * @returns {Promise<{ allowed: boolean, remainingBudget: number, estimatedCost: number }>}
 */
async function checkBudget(costCenter, gpuType, units, hours) {
  const COST_PER_UNIT_HOUR = { T4: 0.5, V100: 2.0, A100: 4.0, H100: 8.0 };
  const estimatedCost = (COST_PER_UNIT_HOUR[gpuType] || 4.0) * units * hours;

  // Production: const res = await fetch(`${CONFIG.FINOPS_API_URL}/budget/${costCenter}`);
  //             const budget = await res.json();
  const mockBudget = { remaining: 2000 };

  return {
    allowed:        mockBudget.remaining >= estimatedCost,
    remainingBudget: mockBudget.remaining,
    estimatedCost,
  };
}

// ── Approval Notification ─────────────────────────────────────────────────────
/**
 * Sends an approval request notification (Slack / Teams / generic webhook).
 * @param {Object} claim
 * @param {Object} budgetInfo
 * @returns {Promise<{ approvalId: string }>}
 */
async function requestApproval(claim, budgetInfo) {
  const payload = {
    text: `GPU Environment Approval Request`,
    details: {
      name:          claim.metadata.name,
      owner:         claim.spec.owner,
      costCenter:    claim.spec.costCenter,
      gpuType:       claim.spec.gpuType,
      units:         claim.spec.units,
      estimatedCost: budgetInfo.estimatedCost.toFixed(2),
      remaining:     budgetInfo.remainingBudget.toFixed(2),
    },
    actions: ["APPROVE", "REJECT"],
  };

  // Production: await fetch(CONFIG.NOTIFY_WEBHOOK, { method: "POST", body: JSON.stringify(payload) });
  console.log("[Notify] Approval request payload:", JSON.stringify(payload, null, 2));
  return { approvalId: `approval-${claim.metadata.name}-${Date.now()}` };
}

// ── Kubernetes Claim Creation ─────────────────────────────────────────────────
/**
 * Creates the XGPUEnvironment Claim in Kubernetes via the API server.
 * @param {Object} claim
 * @returns {Promise<Object>} the created manifest
 */
async function createK8sClaim(claim) {
  const manifest = {
    apiVersion: "platform.gpu-platform.io/v1alpha1",
    kind:       "GPUEnvironment",
    metadata: {
      name:      claim.metadata.name,
      namespace: claim.metadata.namespace || "gpu-self-service",
      labels: {
        "gpu-platform.io/cost-center":  claim.spec.costCenter,
        "gpu-platform.io/owner":        claim.spec.owner,
        "gpu-platform.io/created-by":   "approval-workflow",
      },
      annotations: {
        "platform.gpu-platform.io/approved-at": new Date().toISOString(),
      },
    },
    spec: claim.spec,
  };

  // Production: POST to K8s API with ServiceAccount Bearer token
  console.log("[K8s] Creating claim:", JSON.stringify(manifest, null, 2));
  return manifest;
}

// ── CMDB Registration ─────────────────────────────────────────────────────────
/**
 * Registers the new GPU environment in the CMDB for asset tracking and auditing.
 * @param {Object} claim
 */
async function registerInCMDB(claim) {
  const entry = {
    type:       "GPUEnvironment",
    name:       claim.metadata.name,
    costCenter: claim.spec.costCenter,
    owner:      claim.spec.owner,
    gpuType:    claim.spec.gpuType,
    units:      claim.spec.units,
    createdAt:  new Date().toISOString(),
  };
  // Production: POST to CONFIG.CMDB_API_URL/assets
  console.log("[CMDB] Registering:", JSON.stringify(entry, null, 2));
}

// ── Main Workflow Entry Point ─────────────────────────────────────────────────
/**
 * runApprovalWorkflow orchestrates the full provisioning lifecycle.
 * Called by the workflow engine with the incoming GPUEnvironment claim JSON.
 * @param {Object} claim
 * @returns {Promise<{ status: string, claimName: string }>}
 */
async function runApprovalWorkflow(claim) {
  console.log(`[Workflow] Starting: ${claim.metadata.name}`);

  // Step 1: Compliance validation
  const { valid, errors } = validateClaim(claim);
  if (!valid) {
    throw new Error(`Compliance validation failed:\n${errors.join("\n")}`);
  }
  console.log("[Workflow] Validation passed.");

  // Step 2: Budget check
  const budget = await checkBudget(
    claim.spec.costCenter,
    claim.spec.gpuType,
    claim.spec.units,
    claim.spec.autoExpireHours || 24
  );
  if (!budget.allowed) {
    throw new Error(
      `Insufficient budget for ${claim.spec.costCenter}. ` +
      `Required: $${budget.estimatedCost.toFixed(2)}, Available: $${budget.remainingBudget.toFixed(2)}`
    );
  }
  console.log("[Workflow] Budget OK. Est. cost: $" + budget.estimatedCost.toFixed(2));

  // Step 3: Approval gate
  if (claim.spec.units <= CONFIG.MAX_UNITS_WITHOUT_APPROVAL) {
    console.log(`[Workflow] Auto-approved (≤${CONFIG.MAX_UNITS_WITHOUT_APPROVAL} units).`);
  } else {
    const { approvalId } = await requestApproval(claim, budget);
    console.log(`[Workflow] Approval requested. id=${approvalId}`);
    // In Argo Workflows: suspend here and resume on callback.
    // For demo, approval is assumed granted.
    console.log("[Workflow] Approval granted (simulated).");
  }

  // Step 4: Create K8s claim
  const created = await createK8sClaim(claim);
  console.log("[Workflow] K8s claim created:", created.metadata.name);

  // Step 5: CMDB registration
  await registerInCMDB(claim);
  console.log("[Workflow] CMDB registration complete.");

  return { status: "PROVISIONED", claimName: claim.metadata.name };
}

// ── Module Exports & CLI Runner ───────────────────────────────────────────────
if (typeof module !== "undefined") {
  module.exports = { runApprovalWorkflow, validateClaim, checkBudget };
}

// Local smoke-test: node approval_workflow.js
if (require.main === module) {
  const testClaim = {
    metadata: { name: "test-gpu-env", namespace: "gpu-self-service" },
    spec: {
      gpuType:         "A100",
      units:           2,
      costCenter:      "CC-4210",
      owner:           "platform-team",
      priority:        7,
      autoExpireHours: 8,
    },
  };
  runApprovalWorkflow(testClaim)
    .then((r) => console.log("[Done]", r))
    .catch((e) => { console.error("[Error]", e.message); process.exit(1); });
}
