//go:build windows

package radikron

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func hideFFmpegConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
