// Package main implements a Crossplane Composition Function for GPU sizing.
//
// A Crossplane Function runs as a gRPC server inside a container. The Composition
// Engine calls RunFunction for each pipeline step, passing the observed composite
// resource state and expecting a desired state in return.
//
// This function:
//  1. Reads the XGPUEnvironment spec (gpuType, units, costCenter, priority).
//  2. Validates units against a per-GPU-type tier table.
//  3. Computes estimated cost and auto-expiry timestamp.
//  4. Annotates the composite resource with computed values for downstream steps.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// gpuTier defines per-GPU-type capacity and cost constraints.
type gpuTier struct {
	maxUnits        int
	costPerUnitHour float64 // USD per GPU-unit-hour (adjust to your currency)
}

// tierTable maps GPU model → sizing constraints.
// Adjust costPerUnitHour to reflect your infrastructure cost model.
var tierTable = map[string]gpuTier{
	"T4":   {maxUnits: 16, costPerUnitHour: 0.50},
	"V100": {maxUnits: 8, costPerUnitHour: 2.00},
	"A100": {maxUnits: 8, costPerUnitHour: 4.00},
	"H100": {maxUnits: 4, costPerUnitHour: 8.00},
}

// FunctionRunner implements the Crossplane FunctionRunnerService gRPC interface.
type FunctionRunner struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer
	log *slog.Logger
}

func (f *FunctionRunner) RunFunction(
	_ context.Context,
	req *fnv1beta1.RunFunctionRequest,
) (*fnv1beta1.RunFunctionResponse, error) {
	xrName := req.GetObserved().GetComposite().GetResource().GetMetadata().GetName()
	f.log.Info("RunFunction called", "xr", xrName)

	specFields := req.GetObserved().GetComposite().GetResource().GetSpec().GetFields()

	gpuType := strField(specFields, "gpuType", "A100")
	units := int(numField(specFields, "units", 1))
	costCenter := strField(specFields, "costCenter", "CC-0000")
	priority := int(numField(specFields, "priority", 5))
	expireHours := int(numField(specFields, "autoExpireHours", 24))

	tier, ok := tierTable[gpuType]
	if !ok {
		return fatalResponse(fmt.Sprintf("unsupported gpuType %q; allowed: T4, V100, A100, H100", gpuType))
	}
	if units < 1 {
		units = 1
	}
	if units > tier.maxUnits {
		return fatalResponse(fmt.Sprintf(
			"%d units exceeds maximum %d for GPU type %s", units, tier.maxUnits, gpuType))
	}

	expiresAt := time.Now().UTC().Add(time.Duration(expireHours) * time.Hour).Format(time.RFC3339)
	estimatedCost := float64(units) * tier.costPerUnitHour * float64(expireHours)

	f.log.Info("sizing computed",
		"gpuType", gpuType,
		"units", units,
		"costCenter", costCenter,
		"priority", priority,
		"expiresAt", expiresAt,
		"estimatedCost", estimatedCost,
	)

	// Start from observed state and annotate the composite resource.
	resp := &fnv1beta1.RunFunctionResponse{
		Desired: req.GetObserved(),
		Results: []*fnv1beta1.Result{{
			Severity: fnv1beta1.Severity_SEVERITY_NORMAL,
			Message:  fmt.Sprintf("GPU sizing: %d×%s, est. $%.2f", units, gpuType, estimatedCost),
		}},
	}

	annotations := resp.GetDesired().GetComposite().GetResource().GetMetadata().GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["platform.gpu-platform.io/expires-at"] = expiresAt
	annotations["platform.gpu-platform.io/cost-center"] = costCenter
	annotations["platform.gpu-platform.io/estimated-cost"] = fmt.Sprintf("%.2f", estimatedCost)
	annotations["platform.gpu-platform.io/gpu-priority"] = fmt.Sprintf("%d", priority)

	return resp, nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := ":9443"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen failed", "error", err)
		os.Exit(1)
	}
	srv := grpc.NewServer()
	fnv1beta1.RegisterFunctionRunnerServiceServer(srv, &FunctionRunner{log: log})
	reflection.Register(srv)
	log.Info("GPU Sizing Function started", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		log.Error("gRPC server error", "error", err)
		os.Exit(1)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func strField(fields map[string]interface{}, key, def string) string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func numField(fields map[string]interface{}, key string, def float64) float64 {
	if v, ok := fields[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return def
}

func fatalResponse(msg string) (*fnv1beta1.RunFunctionResponse, error) {
	return &fnv1beta1.RunFunctionResponse{
		Results: []*fnv1beta1.Result{{
			Severity: fnv1beta1.Severity_SEVERITY_FATAL,
			Message:  msg,
		}},
	}, nil
}
