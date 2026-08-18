// Package engine exposes a REST API that executes JavaScript workflow scripts
// using the goja ECMAScript engine (pure Go, no Node.js dependency).
//
// This design deliberately avoids binding the platform to a specific workflow
// vendor (Argo, Temporal, n8n). Any HTTP client – including a Crossplane Function,
// a K8s Job, or a CI pipeline step – can trigger a JS workflow by POST-ing a JSON
// payload to /workflows/{name}/run.
package engine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dop251/goja"
)

// WorkflowEngine loads and executes JavaScript workflow scripts via goja.
type WorkflowEngine struct {
	scriptDir string
	log       *slog.Logger
}

// NewWorkflowEngine creates an engine that reads scripts from scriptDir.
func NewWorkflowEngine(scriptDir string) *WorkflowEngine {
	return &WorkflowEngine{
		scriptDir: scriptDir,
		log:       slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// RunRequest is the payload expected by the /workflows/{name}/run endpoint.
type RunRequest struct {
	// Input is the arbitrary JSON object passed to the workflow as its first argument.
	Input json.RawMessage `json:"input"`
}

// RunResult is the response returned after workflow execution.
type RunResult struct {
	Script     string      `json:"script"`
	Output     interface{} `json:"output,omitempty"`
	Error      string      `json:"error,omitempty"`
	DurationMs int64       `json:"durationMs"`
}

// ServeHTTP registers routes and starts the HTTP server.
func (e *WorkflowEngine) ServeHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/workflows/", e.handleWorkflow)
	e.log.Info("Workflow Engine starting", "addr", addr, "scriptDir", e.scriptDir)
	return http.ListenAndServe(addr, mux)
}

// handleWorkflow handles POST /workflows/{name}/run.
func (e *WorkflowEngine) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST is supported", http.StatusMethodNotAllowed)
		return
	}

	// Extract workflow name from URL: /workflows/{name}/run
	name := filepath.Base(filepath.Dir(r.URL.Path))
	if name == "" || name == "." {
		http.Error(w, "workflow name is required", http.StatusBadRequest)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	result, err := e.execute(name, req.Input)
	if err != nil {
		e.log.Error("workflow execution failed", "name", name, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(RunResult{Script: name, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// execute loads the named script file and runs it inside an isolated goja VM.
func (e *WorkflowEngine) execute(name string, input json.RawMessage) (*RunResult, error) {
	scriptPath := filepath.Join(e.scriptDir, name+".js")
	src, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("script not found: %w", err)
	}

	start := time.Now()

	vm := goja.New()

	// Provide a console.log shim so scripts can log to stdout.
	console := vm.NewObject()
	_ = console.Set("log", func(args ...interface{}) {
		e.log.Info("[JS]", "script", name, "msg", fmt.Sprint(args...))
	})
	_ = console.Set("error", func(args ...interface{}) {
		e.log.Error("[JS]", "script", name, "msg", fmt.Sprint(args...))
	})
	_ = vm.Set("console", console)

	// Inject require stub (scripts use CommonJS module.exports pattern).
	_ = vm.Set("require", func(mod string) interface{} {
		e.log.Info("[JS] require() called (stub)", "module", mod)
		return goja.Undefined()
	})
	_ = vm.Set("module", map[string]interface{}{"exports": map[string]interface{}{}})
	_ = vm.Set("process", map[string]interface{}{
		"env": map[string]interface{}{
			"FINOPS_API_URL": os.Getenv("FINOPS_API_URL"),
			"SLACK_WEBHOOK":  os.Getenv("SLACK_WEBHOOK"),
			"K8S_API_URL":    os.Getenv("K8S_API_URL"),
		},
		"main": nil, // disable CLI auto-run block
	})

	// Parse and run the script.
	_, err = vm.RunScript(name+".js", string(src))
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	// Retrieve the exported runApprovalWorkflow function.
	exports := vm.Get("module").Export().(map[string]interface{})
	runFn, ok := exports["runApprovalWorkflow"]
	if !ok {
		return nil, fmt.Errorf("script does not export runApprovalWorkflow")
	}

	fn, ok := runFn.(func(goja.FunctionCall) goja.Value)
	if !ok {
		return nil, fmt.Errorf("runApprovalWorkflow is not callable")
	}

	// Unmarshal input into a goja-compatible object.
	var inputMap interface{}
	_ = json.Unmarshal(input, &inputMap)
	inputVal := vm.ToValue(inputMap)

	// Call the function (synchronous execution; async support via promise extension).
	result := fn(goja.FunctionCall{Arguments: []goja.Value{inputVal}})

	elapsed := time.Since(start)
	return &RunResult{
		Script:     name,
		Output:     result.Export(),
		DurationMs: elapsed.Milliseconds(),
	}, nil
}
