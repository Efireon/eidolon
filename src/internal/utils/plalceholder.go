package utils

import (
	"eidolonVPN/internal/errors/handlers"
	"fmt"
	"os/exec"
	"strconv"
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

func ChmodFile(filepath string, permissions interface{}) error {
	switch v := permissions.(type) {
	case string:
		cmd := exec.Command("chmod", "+x", filepath)
		if err := cmd.Run(); err != nil {
			return handlers.UtilsErrHandler(filepath, err)
		}
		DebugPrint(fmt.Sprintf("Chmod +x successfully executed for: %s", filepath))

	case int:
		cmd := exec.Command("chmod", strconv.Itoa(v), filepath)
		if err := cmd.Run(); err != nil {
			return handlers.UtilsErrHandler(filepath, err)
		}
		DebugPrint(fmt.Sprintf("File %s permissions successfully changed to: %v", filepath, v))

	default:
		return fmt.Errorf("unexpected permissions type: %T", v)
	}
	return nil
}
