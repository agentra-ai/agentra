//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const createNewProcessGroup = 0x00000200

func configureBackgroundProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

func terminateProcess(process *os.Process) error {
	// Windows does not provide a portable SIGTERM equivalent for arbitrary
	// console processes. Kill is bounded to the daemon PID reported by its
	// authenticated local health endpoint.
	return process.Kill()
}

func daemonSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
