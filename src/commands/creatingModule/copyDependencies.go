package creatingModule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CopyDependencies(binary, newRoot string) error {
	cmd := exec.Command("ldd", binary)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.HasPrefix(field, "/") {
				dest := filepath.Join(newRoot, field)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					return err
				}
				if err := CopyFile(field, dest); err != nil {
					return err
				}
				if err := os.Chmod(dest, 0755); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
