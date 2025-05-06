package main

import (
	"eidolonVPN/internal/config"
	"eidolonVPN/internal/config/structures"
	"eidolonVPN/internal/errors/handlers"
	"eidolonVPN/internal/openconnect"
	"eidolonVPN/internal/utils"
	"os"

	"fmt"
	"log"
)

// Создаю переменнцю с путями конфигов TODO: Сделать более корректное сохранение путей
var paths = []string{
	"/eidolon/service/config",
}

func main() {
	var mainConfig structures.MainConfig
	err := config.LoadConfig("main", paths, &mainConfig)
	if err != nil {
		log.Fatalf("Critical: failed to load main config: %v", err)
	}

	OCconfig, err := openconnect.SearchOCconfig("/eidolon/service/ocserv/ocserv.conf")
	if err != nil {
		// Создаем директорию, если ее нет
		os.MkdirAll("/etc/ocserv", 0755)

		// Генерируем конфигурацию
		err = openconnect.GenerateOCconfig("/eidolon/service/config", "/eidolon/service/ocserv/ocserv.conf")
		if err != nil {
			handlers.OpenConnectFileErrHandler(OCconfig, err)
		} else {
			utils.DebugPrint("Generated new OpenConnect configuration")
		}
	}

	// Времнные дебаги для теста контейнера
	utils.DebugPrint(fmt.Sprintf("Hello, %s!", utils.СmdExec("whoami")))
	utils.DebugPrint(fmt.Sprintf("Debug: %s", utils.СmdExec("uname -r")))
	utils.DebugPrint(fmt.Sprintf("Main Config: %+v", mainConfig))

	utils.DebugPrint(fmt.Sprintf("Service config containment: %s", mainConfig.Service.Host))

	utils.DebugPrint(fmt.Sprintf("OCconfig: %s", OCconfig))

}
