package sender

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

// Config holds SMTP settings for sending emails.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SendToKindle sends a book file to a Kindle email address.
func SendToKindle(cfg Config, toEmail, filePath string) error {
	if cfg.Host == "" || cfg.From == "" || toEmail == "" {
		return fmt.Errorf("SMTP not configured or no recipient email")
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	fileName := filepath.Base(filePath)

	// Build MIME email with attachment.
	boundary := "coppermind-boundary-12345"
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", fileName))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))

	// Text body.
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString("Sent from Coppermind.\r\n\r\n")

	// Attachment.
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString(fmt.Sprintf("Content-Type: application/octet-stream; name=%q\r\n", fileName))
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%q\r\n\r\n", fileName))
	msg.WriteString(base64.StdEncoding.EncodeToString(fileData))
	msg.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	return smtp.SendMail(addr, auth, cfg.From, []string{toEmail}, []byte(msg.String()))
}
