//go:build windows

package process

import (
	"os/exec"
	"strconv"
)

func configureProcessTree(command *exec.Cmd) { command.SysProcAttr = nil }

// taskkill /T is the Windows-native process-tree boundary. It is invoked by
// argv list and never interpolates worker/media input into a shell command.
func killProcessTree(pid int) error {
	return exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
