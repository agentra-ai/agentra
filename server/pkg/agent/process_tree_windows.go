//go:build windows

package agent

import (
	"os"
	"os/exec"
	"strconv"
	"time"
)

// configureProcessTree uses the Windows taskkill tree operation on context
// cancellation. taskkill is available on supported Windows releases and
// recursively terminates provider children before Cmd.Wait returns.
func configureProcessTree(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		killer := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		if err := killer.Run(); err == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
}
