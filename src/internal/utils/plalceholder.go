package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

func СmdExec(msg string) string {
	parts := strings.Split(msg, " ")
	if len(parts) == 0 {
		return fmt.Sprintf("Error, out of bounds. %v", len(parts))
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Command error: %v", err)
	} else {
		return strings.TrimSpace(string(out))
	}
}

func DebugPrint(msg string) {
	fmt.Println(msg)
}
