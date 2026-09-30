//go:build !windows

package radikron

import "os/exec"

func hideFFmpegConsole(*exec.Cmd) {}
