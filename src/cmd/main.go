package main

import (
	"eidolonVPN/internal/utils"
	"fmt"
)

func main() {
	utils.DebugPrint(fmt.Sprintf("Hello, %s!", utils.СmdExec("whoami")))
	utils.DebugPrint(fmt.Sprintf("Debug: %s", utils.СmdExec("uname -r")))
}
