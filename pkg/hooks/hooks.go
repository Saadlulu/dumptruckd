// Package hooks provides pre/post backup hook execution.
package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DefaultTimeout is the default timeout for hook command execution.
const DefaultTimeout = 60 * time.Second

// RunHook executes a shell command with a configurable timeout and injected
// environment variables. The command is run via "sh -c" to support shell
// features like pipes and redirects. The parent process environment is
// inherited, and the provided env map is added on top.
//
// The caller is expected to populate env with keys such as
// DUMPTRUCKD_HOOK_BACKUP_NAME, DUMPTRUCKD_HOOK_STATUS, and
// DUMPTRUCKD_HOOK_FILE_PATH.
func RunHook(ctx context.Context, command string, env map[string]string, timeout time.Duration) error {
	if command == "" {
		return nil
	}

	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// Inherit parent environment and layer hook-specific vars on top.
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("hook timed out after %s: %w", timeout, ctx.Err())
		}
		return fmt.Errorf("hook command failed: %w\noutput: %s", err, string(output))
	}

	return nil
}
