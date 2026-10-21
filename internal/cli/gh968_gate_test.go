package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#968 — `skrptiq run` resumed a gate from INSIDE the runner's progress
// callback, which fires before the runner writes status='paused'. The engine
// refused it ("execution is not paused (status: running)"), the error was
// discarded by a bare `go engine.ResumeExecution(...)`, and the run reported
// status="completed" with exit 0 and empty output while the execution sat
// paused with its post-gate step never started. A false success, not a display
// bug: these tests assert the ABSENCE of that silent failure.

func gateStrp(s string) *string { return &s }
func gateIntp(i int) *int       { return &i }

// setupGatedDB builds a workflow of a pure-review gate followed by a
// deterministic local.template step — zero tokens, no network, so the gate path
// is testable without provider keys.
func setupGatedDB(t *testing.T) string {
	t.Helper()
	return writeWorkflowDB(t, `{"execution":[{"skill":"review-gate","gate":true},{"skill":"after-gate","step_type":"local.template"}]}`,
		[]nodeSpec{
			{"g1", "Review Gate", "Check this", "review-gate"},
			{"s2", "After Gate", "POST-GATE-OUTPUT", "after-gate"},
		})
}

// setupTwoGateDB builds gate → template → gate → template, so the resume loop
// has to survive a resume that itself comes back awaiting input.
func setupTwoGateDB(t *testing.T) string {
	t.Helper()
	return writeWorkflowDB(t, `{"execution":[{"skill":"gate-one","gate":true},{"skill":"mid","step_type":"local.template"},{"skill":"gate-two","gate":true},{"skill":"tail","step_type":"local.template"}]}`,
		[]nodeSpec{
			{"g1", "Gate One", "First check", "gate-one"},
			{"m1", "Mid Step", "MID-OUTPUT", "mid"},
			{"g2", "Gate Two", "Second check", "gate-two"},
			{"t1", "Tail Step", "TAIL-OUTPUT", "tail"},
		})
}

type nodeSpec struct {
	id, title, content, slug string
}

func writeWorkflowDB(t *testing.T, meta string, nodes []nodeSpec) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gated.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.CreateNode("wf", "workflow", "Gated WF", nil, nil, gateStrp(meta), gateStrp("gated-wf"), nil); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	for i, n := range nodes {
		if err := db.CreateNode(n.id, "skill", n.title, nil, gateStrp(n.content), nil, gateStrp(n.slug), nil); err != nil {
			t.Fatalf("create node %s: %v", n.id, err)
		}
		if err := db.CreateEdge("e"+n.id, "wf", n.id, "uses", gateIntp(i+1)); err != nil {
			t.Fatalf("create edge %s: %v", n.id, err)
		}
	}
	return dbPath
}

func stepStatuses(t *testing.T, dbPath, execID string) (string, map[string]string) {
	t.Helper()
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	e, err := db.GetExecution(execID)
	if err != nil || e == nil {
		t.Fatalf("load execution %q: %v", execID, err)
	}
	steps, err := db.GetStepsByExecutionGlobal(execID)
	if err != nil {
		t.Fatalf("load steps: %v", err)
	}
	byNode := make(map[string]string, len(steps))
	for _, s := range steps {
		byNode[s.NodeID] = s.Status
	}
	return e.Status, byNode
}

// The headline regression: --yes must actually resume, run the post-gate step,
// and earn its exit 0.
func TestGH968_AutoApprovedGate_RunsPostGateStep(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateAutoApprove, 0, &lastOutput, &execID, nil)

	if res.status != "completed" || res.exitCode != ExitOK {
		t.Errorf("auto-approved gated run: got status=%q exit=%d, want completed/%d", res.status, res.exitCode, ExitOK)
	}
	if !strings.Contains(lastOutput, "POST-GATE-OUTPUT") {
		t.Errorf("post-gate output missing from run output: %q", lastOutput)
	}
	// The gate step's own output is progress, not product — it must not be piped.
	if strings.Contains(lastOutput, "Check this") {
		t.Errorf("gate text leaked into the run output: %q", lastOutput)
	}
	status, steps := stepStatuses(t, dbPath, execID)
	if status != "completed" {
		t.Errorf("execution row status = %q, want completed", status)
	}
	if steps["s2"] != "completed" {
		t.Errorf("post-gate step status = %q, want completed", steps["s2"])
	}
}

// Two gates: the resume loop must handle a resume that itself pauses.
func TestGH968_TwoSequentialGates_BothResume(t *testing.T) {
	dbPath := setupTwoGateDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateAutoApprove, 0, &lastOutput, &execID, nil)

	if res.status != "completed" || res.exitCode != ExitOK {
		t.Errorf("two-gate run: got status=%q exit=%d, want completed/%d", res.status, res.exitCode, ExitOK)
	}
	for _, want := range []string{"MID-OUTPUT", "TAIL-OUTPUT"} {
		if !strings.Contains(lastOutput, want) {
			t.Errorf("output missing %q (both gates must resume): %q", want, lastOutput)
		}
	}
	status, steps := stepStatuses(t, dbPath, execID)
	if status != "completed" {
		t.Errorf("execution row status = %q, want completed", status)
	}
	if steps["t1"] != "completed" {
		t.Errorf("step after the SECOND gate = %q, want completed", steps["t1"])
	}
}

// --strict must decline through RejectGate — the execution ends 'cancelled',
// never left 'paused' — and must NOT exit 0.
func TestGH968_StrictGate_DeclinesAndExitsNonZero(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateStrict, 0, &lastOutput, &execID, nil)

	if res.exitCode == ExitOK {
		t.Errorf("a rejected gate must not exit 0 (got status=%q exit=%d)", res.status, res.exitCode)
	}
	if res.exitCode != ExitFailed {
		t.Errorf("rejected gate exit code = %d, want %d", res.exitCode, ExitFailed)
	}
	status, steps := stepStatuses(t, dbPath, execID)
	if status != "cancelled" {
		t.Errorf("declined execution status = %q, want cancelled (never left paused)", status)
	}
	if steps["g1"] != "skipped" {
		t.Errorf("declined gate step = %q, want skipped", steps["g1"])
	}
	if steps["s2"] == "completed" {
		t.Error("post-gate step ran despite the gate being declined")
	}
}

// An interactive "n" declines the same way.
func TestGH968_InteractiveReject_ExitsNonZero(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	restore := feedStdin(t, "n\n")
	defer restore()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateDefault, 0, &lastOutput, &execID, nil)

	if res.exitCode != ExitFailed {
		t.Errorf("interactive reject exit = %d, want %d", res.exitCode, ExitFailed)
	}
	status, _ := stepStatuses(t, dbPath, execID)
	if status != "cancelled" {
		t.Errorf("status after reject = %q, want cancelled", status)
	}
}

// An unanswered gate is a timeout, distinct from a typed "no".
func TestGH968_GateTimeout_DeclinesWithTimeoutCode(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	// A pipe with nothing written: the read blocks until the timeout fires.
	restore := feedStdin(t, "")
	defer restore()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateDefault, 1, &lastOutput, &execID, nil)

	if res.exitCode != ExitTimeout {
		t.Errorf("timed-out gate exit = %d, want %d", res.exitCode, ExitTimeout)
	}
	status, steps := stepStatuses(t, dbPath, execID)
	if status != "cancelled" {
		t.Errorf("status after timeout = %q, want cancelled", status)
	}
	if steps["s2"] == "completed" {
		t.Error("post-gate step ran despite the gate timing out")
	}
}

// A resume that errors must surface as failure. This is the shape of the
// original bug: the error existed and nobody saw it.
func TestGH968_ResumeError_SurfacesAsFailure(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = pr
	defer func() { os.Stdin = orig; pr.Close() }()

	// Break the resume while the gate prompt is blocked on stdin: dropping the
	// workflow node makes the plan rebuild inside ResumeExecution fail. Then
	// answer "y" so the resume is attempted and must report the error.
	go func() {
		defer pw.Close()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			db, derr := storage.Open(dbPath)
			if derr == nil {
				var status string
				row := db.QueryRow("SELECT status FROM executions LIMIT 1")
				if row.Scan(&status) == nil && status == "paused" {
					db.Exec("DELETE FROM nodes WHERE id = 'wf'")
					db.Close()
					break
				}
				db.Close()
			}
			time.Sleep(20 * time.Millisecond)
		}
		pw.Write([]byte("y\n"))
	}()

	var lastOutput, execID string
	res := runWorkflow(context.Background(), engine, "wf", map[string]string{}, gateDefault, 0, &lastOutput, &execID, nil)

	if res.status != "failed" || res.exitCode != ExitFailed {
		t.Errorf("a failed resume must report failure: got status=%q exit=%d err=%q", res.status, res.exitCode, res.err)
	}
	if res.err == "" {
		t.Error("resume error was swallowed — res.err is empty")
	}
	t.Logf("resume failure reported as: %q", res.err)
}

// A run left paused (nothing approved the gate) must not report completed.
func TestGH968_PausedRun_DoesNotReportCompleted(t *testing.T) {
	dbPath := setupGatedDB(t)
	engine := OpenEngine(dbPath)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	// Drive the pre-gate leg with a callback that records the execution and
	// resumes nothing, leaving the row paused — then ask for the verdict.
	var execID string
	if _, err := engine.RunWorkflow(context.Background(), "wf", map[string]string{}, func(evt exec.ProgressEvent) {
		if evt.Type == "execution-started" {
			execID = evt.ExecutionID
		}
	}); err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	res := finalRunStatus(engine, execID, "", nil)
	if res.exitCode == ExitOK || res.status == "completed" {
		t.Errorf("a paused run reported success: status=%q exit=%d", res.status, res.exitCode)
	}
	if res.status != "paused" {
		t.Errorf("status = %q, want paused", res.status)
	}
}

// Guard: run and resume must share ONE renderer. Re-adding an inline callback at
// a resume call site is exactly how post-gate silence regresses.
func TestGH968_RendererParity_NoInlineResumeCallback(t *testing.T) {
	src, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatalf("read run.go: %v", err)
	}
	text := string(src)

	if strings.Contains(text, "exec.ProgressEvent) {}") {
		t.Error("run.go passes an empty ProgressEvent callback — post-gate events would be discarded (GH#968)")
	}
	for _, call := range []string{"engine.ResumeExecution(", "engine.RejectGate("} {
		idx := strings.Index(text, call)
		if idx < 0 {
			t.Errorf("%s call site missing", call)
			continue
		}
		args := text[idx : idx+strings.Index(text[idx:], ")")+1]
		if !strings.Contains(args, "onProgress") {
			t.Errorf("%s does not thread the shared onProgress renderer: %s", call, args)
		}
	}
	if strings.Contains(text, "go engine.ResumeExecution(") {
		t.Error("run.go resumes in a detached goroutine — the process exits before the resume finishes (GH#968)")
	}
	if strings.Contains(text, "engine.StopExecution(") {
		t.Error("run.go still uses StopExecution for a gate decline — it is a no-op on a paused execution; use RejectGate")
	}
}

// feedStdin replaces os.Stdin with a pipe carrying s, returning a restore func.
func feedStdin(t *testing.T, s string) func() {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if s != "" {
		go func() {
			pw.Write([]byte(s))
			pw.Close()
		}()
	}
	orig := os.Stdin
	os.Stdin = pr
	return func() {
		os.Stdin = orig
		pr.Close()
		if s == "" {
			pw.Close()
		}
	}
}
