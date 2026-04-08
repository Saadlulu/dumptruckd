package hooks

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunHook_EmptyCommand(t *testing.T) {
	t.Parallel()
	err := RunHook(context.Background(), "", nil, DefaultTimeout)
	if err != nil {
		t.Errorf("RunHook() with empty command = %v, want nil", err)
	}
}

func TestRunHook_SuccessfulCommand(t *testing.T) {
	t.Parallel()
	err := RunHook(context.Background(), "true", nil, DefaultTimeout)
	if err != nil {
		t.Errorf("RunHook() = %v, want nil", err)
	}
}

func TestRunHook_NonZeroExitCode(t *testing.T) {
	t.Parallel()
	err := RunHook(context.Background(), "exit 1", nil, DefaultTimeout)
	if err == nil {
		t.Fatal("RunHook() = nil, want error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "hook command failed") {
		t.Errorf("error = %q, want it to contain 'hook command failed'", err)
	}
}

func TestRunHook_EnvVarInjection(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"DUMPTRUCKD_HOOK_BACKUP_NAME": "prod_db",
		"DUMPTRUCKD_HOOK_STATUS":      "success",
		"DUMPTRUCKD_HOOK_FILE_PATH":   "/tmp/backup.sql.gz",
	}

	// The script checks that all three env vars are set to expected values.
	script := `
		[ "$DUMPTRUCKD_HOOK_BACKUP_NAME" = "prod_db" ] || exit 10
		[ "$DUMPTRUCKD_HOOK_STATUS" = "success" ] || exit 11
		[ "$DUMPTRUCKD_HOOK_FILE_PATH" = "/tmp/backup.sql.gz" ] || exit 12
	`

	err := RunHook(context.Background(), script, env, DefaultTimeout)
	if err != nil {
		t.Errorf("RunHook() = %v, want nil (env vars should be injected)", err)
	}
}

func TestRunHook_Timeout(t *testing.T) {
	t.Parallel()

	err := RunHook(context.Background(), "sleep 10", nil, 100*time.Millisecond)
	if err == nil {
		t.Fatal("RunHook() = nil, want timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want it to contain 'timed out'", err)
	}
}

func TestRunHook_DefaultTimeout(t *testing.T) {
	t.Parallel()

	// Passing 0 should use the default timeout, not block forever.
	// A quick command should succeed regardless.
	err := RunHook(context.Background(), "true", nil, 0)
	if err != nil {
		t.Errorf("RunHook() with zero timeout = %v, want nil", err)
	}
}

func TestRunHook_ShellFeatures(t *testing.T) {
	t.Parallel()

	// Verify pipes work via sh -c.
	err := RunHook(context.Background(), "echo hello | grep hello", nil, DefaultTimeout)
	if err != nil {
		t.Errorf("RunHook() with pipe = %v, want nil", err)
	}
}

func TestRunHook_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := RunHook(ctx, "sleep 10", nil, DefaultTimeout)
	if err == nil {
		t.Fatal("RunHook() = nil, want error for cancelled context")
	}
}
