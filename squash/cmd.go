package squash

import (
	"fmt"
	"os/exec"
)

// runCmd runs name with args, injects env into the process environment,
// and returns stdout. On failure it wraps stderr into the error.
func runCmd(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %s", name, string(ee.Stderr))
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}
