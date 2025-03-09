package vpn

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CertificateManager управляет сертификатами для VPN
type CertificateManager struct {
	caKey         *rsa.PrivateKey
	caCert        *x509.Certificate
	serverKey     *rsa.PrivateKey
	serverCert    *x509.Certificate
	certDirectory string
}

// CertOptions содержит опции для создания сертификата
type CertOptions struct {
	CommonName    string
	Organization  string
	Country       string
	Locality      string
	ValidForDays  int
	KeySize       int
	CertDirectory string
	CertBaseName  string
	IsServer      bool
	IsCA          bool
	CAKeyPath     string
	CACertPath    string
}

// NewCertificateManager создает новый менеджер сертификатов
func NewCertificateManager(certDirectory string) (*CertificateManager, error) {
	manager := &CertificateManager{
		certDirectory: certDirectory,
	}

	// Создаем директорию для сертификатов, если она не существует
	if err := os.MkdirAll(certDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certificate directory: %w", err)
	}

	return manager, nil
}

// LoadOrCreateCA загружает или создает CA сертификат
func (m *CertificateManager) LoadOrCreateCA(options CertOptions) error {
	caKeyPath := filepath.Join(m.certDirectory, "ca.key")
	caCertPath := filepath.Join(m.certDirectory, "ca.crt")

	// Попытка загрузить существующие CA файлы
	caKey, caCert, err := m.loadCertificateAndKey(caKeyPath, caCertPath)
	if err == nil {
		m.caKey = caKey
		m.caCert = caCert
		return nil
	}

	// Создаем новый CA, если файлы не существуют или не удалось загрузить
	options.IsCA = true
	options.CertBaseName = "ca"
	options.IsServer = false

	caCert, caKey, err = m.createCertificate(options)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	m.caCert = caCert
	m.caKey = caKey
	return nil
}

// LoadOrCreateServerCert загружает или создает сертификат сервера
func (m *CertificateManager) LoadOrCreateServerCert(options CertOptions) error {
	if m.caCert == nil || m.caKey == nil {
		return fmt.Errorf("CA certificate not loaded")
	}

	serverKeyPath := filepath.Join(m.certDirectory, "server.key")
	serverCertPath := filepath.Join(m.certDirectory, "server.crt")

	// Попытка загрузить существующие файлы сертификата сервера
	serverKey, serverCert, err := m.loadCertificateAndKey(serverKeyPath, serverCertPath)
	if err == nil {
		m.serverKey = serverKey
		m.serverCert = serverCert
		return nil
	}

	// Создаем новый сертификат сервера
	options.IsCA = false
	options.CertBaseName = "server"
	options.IsServer = true
	options.CAKeyPath = filepath.Join(m.certDirectory, "ca.key")
	options.CACertPath = filepath.Join(m.certDirectory, "ca.crt")

	serverCert, serverKey, err = m.createCertificate(options)
	if err != nil {
		return fmt.Errorf("failed to create server certificate: %w", err)
	}

	m.serverCert = serverCert
	m.serverKey = serverKey
	return nil
}

// CreateClientCertificate создает и сохраняет сертификат клиента
func (m *CertificateManager) CreateClientCertificate(username string, options CertOptions) (string, error) {
	if m.caCert == nil || m.caKey == nil {
		return "", fmt.Errorf("CA certificate not loaded")
	}

	options.IsCA = false
	options.CertBaseName = username
	options.IsServer = false
	options.CommonName = username
	options.CAKeyPath = filepath.Join(m.certDirectory, "ca.key")
	options.CACertPath = filepath.Join(m.certDirectory, "ca.crt")

	cert, key, err := m.createCertificate(options)
	if err != nil {
		return "", fmt.Errorf("failed to create client certificate: %w", err)
	}

	// Кодируем сертификат в PEM формат
	certPEM, err := encodeCertificateToPEM(cert, key)
	if err != nil {
		return "", fmt.Errorf("failed to encode certificate to PEM: %w", err)
	}

	return certPEM, nil
}

// GetCAFilePath возвращает путь к файлу CA сертификата
func (m *CertificateManager) GetCAFilePath() string {
	return filepath.Join(m.certDirectory, "ca.crt")
}

// GetServerCertFilePath возвращает путь к файлу сертификата сервера
func (m *CertificateManager) GetServerCertFilePath() string {
	return filepath.Join(m.certDirectory, "server.crt")
}

// GetServerKeyFilePath возвращает путь к файлу ключа сервера
func (m *CertificateManager) GetServerKeyFilePath() string {
	return filepath.Join(m.certDirectory, "server.key")
}

// loadCertificateAndKey загружает сертификат и ключ из файлов
func (m *CertificateManager) loadCertificateAndKey(keyPath, certPath string) (*rsa.PrivateKey, *x509.Certificate, error) {
	// Проверяем, существуют ли файлы
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("key file does not exist: %w", err)
	}
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("certificate file does not exist: %w", err)
	}

	// Читаем файл ключа
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Декодируем PEM блок ключа
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to parse PEM block containing key")
	}

	// Парсим ключ
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Читаем файл сертификата
	certData, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Декодируем PEM блок сертификата
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to parse PEM block containing certificate")
	}

	// Парсим сертификат
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return key, cert, nil
}

// createCertificate создает новый сертификат и ключ
func (m *CertificateManager) createCertificate(options CertOptions) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Устанавливаем дефолтные значения, если не указаны
	if options.KeySize == 0 {
		options.KeySize = 2048
	}
	if options.ValidForDays == 0 {
		options.ValidForDays = 365
	}
	if options.CertDirectory == "" {
		options.CertDirectory = m.certDirectory
	}

	// Генерируем новую пару ключей RSA
	key, err := rsa.GenerateKey(rand.Reader, options.KeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Создаем шаблон сертификата
	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(options.ValidForDays) * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   options.CommonName,
			Organization: []string{options.Organization},
			Country:      []string{options.Country},
			Locality:     []string{options.Locality},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	if options.IsServer {
		template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	}

	if options.IsCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	var cert []byte
	var parent *x509.Certificate
	var signingKey *rsa.PrivateKey

	if options.IsCA {
		// Самоподписанный сертификат
		parent = &template
		signingKey = key
	} else {
		// Подписанный CA сертификат
		caKey, caCert, err := m.loadCertificateAndKey(options.CAKeyPath, options.CACertPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load CA certificate and key: %w", err)
		}
		parent = caCert
		signingKey = caKey
	}

	// Создаем сертификат
	cert, err = x509.CreateCertificate(rand.Reader, &template, parent, &key.PublicKey, signingKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Сохраняем сертификат и ключ в файлы
	if options.CertBaseName != "" {
		err = saveCertificateAndKey(cert, key, options.CertDirectory, options.CertBaseName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to save certificate and key: %w", err)
		}
	}

	// Парсим сертификат для возврата
	parsedCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse created certificate: %w", err)
	}

	return parsedCert, key, nil
}

// saveCertificateAndKey сохраняет сертификат и ключ в файлы
func saveCertificateAndKey(cert []byte, key *rsa.PrivateKey, directory, baseName string) error {
	// Создаем директорию, если она не существует
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Сохраняем сертификат
	certPath := filepath.Join(directory, baseName+".crt")
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open certificate file for writing: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return fmt.Errorf("failed to write certificate to file: %w", err)
	}

	// Сохраняем ключ
	keyPath := filepath.Join(directory, baseName+".key")
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open key file for writing: %w", err)
	}
	defer keyOut.Close()

	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return fmt.Errorf("failed to write key to file: %w", err)
	}

	return nil
}

// encodeCertificateToPEM кодирует сертификат и ключ в PEM формат
func encodeCertificateToPEM(cert *x509.Certificate, key *rsa.PrivateKey) (string, error) {
	var buf bytes.Buffer

	// Кодируем сертификат
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		return "", fmt.Errorf("failed to encode certificate to PEM: %w", err)
	}

	// Кодируем ключ
	if err := pem.Encode(&buf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return "", fmt.Errorf("failed to encode key to PEM: %w", err)
	}

	return buf.String(), nil
}

// generateSerialNumber генерирует случайный серийный номер для сертификата
func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	return serialNumber, nil
}
