//go:build !unix

package claude

import (
	"fmt"
	"os/exec"
	"runtime"
)

func setProcessUser(cmd *exec.Cmd, username string) error {
	if username == "" {
		return nil
	}
	return fmt.Errorf("user option is unsupported on %s", runtime.GOOS)
}
