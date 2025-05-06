package openconnect

import (
	"eidolonVPN/internal/config"
	"eidolonVPN/internal/config/structures"
	"eidolonVPN/internal/errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Manager управляет процессом OpenConnect
type Manager struct {
	cmd        *exec.Cmd
	config     structures.OpenConnectConfig
	configPath string
	running    bool
	mutex      sync.Mutex
	logWriter  io.Writer
}

// NewManager создает новый экземпляр менеджера OpenConnect
func NewManager(configPath string) (*Manager, error) {
	var ocConfig structures.OpenConnectConfig

	err := config.LoadConfig("openconnect", []string{"/eidolon/service/config"}, &ocConfig)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:     ocConfig,
		configPath: configPath,
		running:    false,
		logWriter:  os.Stdout, // По умолчанию логи в stdout
	}, nil
}

// Start запускает процесс OpenConnect
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return errors.CallOpenConnectError("Process already running", nil)
	}

	// Проверяем наличие конфигурации
	exists, err := CheckOCconfig(m.configPath)
	if err != nil || !exists {
		// Генерируем конфигурацию, если она не существует или неверна
		err = GenerateOCconfig("/eidolon/service/config", m.configPath)
		if err != nil {
			return err
		}
	}

	// Формируем команду запуска
	m.cmd = exec.Command("openconnect", "--config", m.configPath)

	// Настраиваем вывод логов
	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return errors.CallOpenConnectError("Failed to get stdout pipe", err)
	}

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return errors.CallOpenConnectError("Failed to get stderr pipe", err)
	}

	// Объединяем stdout и stderr в один writer
	go io.Copy(m.logWriter, stdout)
	go io.Copy(m.logWriter, stderr)

	// Запускаем процесс
	err = m.cmd.Start()
	if err != nil {
		return errors.CallOpenConnectError("Failed to start OpenConnect", err)
	}

	m.running = true

	// Запускаем горутину для отслеживания завершения процесса
	go func() {
		err := m.cmd.Wait()
		m.mutex.Lock()
		m.running = false
		m.mutex.Unlock()

		if err != nil {
			fmt.Printf("OpenConnect process exited with error: %v\n", err)
		} else {
			fmt.Println("OpenConnect process exited normally")
		}
	}()

	return nil
}

// Stop останавливает процесс OpenConnect
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return errors.CallOpenConnectError("Process not running", nil)
	}

	// Посылаем SIGTERM для graceful shutdown
	err := m.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		// Если не удалось послать SIGTERM, принудительно завершаем
		err = m.cmd.Process.Kill()
		if err != nil {
			return errors.CallOpenConnectError("Failed to kill process", err)
		}
	}

	// Статус обновится в горутине Wait
	return nil
}

// IsRunning проверяет, запущен ли процесс
func (m *Manager) IsRunning() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.running
}

// SetLogWriter устанавливает writer для вывода логов
func (m *Manager) SetLogWriter(writer io.Writer) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.logWriter = writer
}
