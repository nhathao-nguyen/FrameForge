//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func configureProcessTree(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }
