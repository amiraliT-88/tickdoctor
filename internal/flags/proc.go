package flags

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ReadProcessCommandLine extracts the startup commandline of a running process by PID.
func ReadProcessCommandLine(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("invalid pid: %d", pid)
	}

	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			return "", err
		}
		parts := strings.Split(string(data), "\x00")
		return strings.Join(parts, " "), nil
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("(Get-CimInstance Win32_Process -Filter 'ProcessId = %d').CommandLine", pid))
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to query process %d: %w", pid, err)
		}
		res := strings.TrimSpace(string(out))
		if res == "" {
			return "", fmt.Errorf("process %d not found or command line is inaccessible", pid)
		}
		return res, nil
	}

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}
