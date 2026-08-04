//go:build darwin || linux

package agent

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// configureProcessTree isolates the runtime in its own process group. Context
// cancellation then kills the whole group, including shells and tools started
// by the provider CLI, instead of leaving descendants behind.
func configureProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
