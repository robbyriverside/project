package finalize

import (
	"fmt"
	"os"
	"os/exec"
)

func InitMod(dir, module string) error {
	cmd := exec.Command("go", "mod", "init", module)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod init failed: %w", err)
	}
	return nil
}

func Tidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}
	return nil
}
