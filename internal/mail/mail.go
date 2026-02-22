package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// Note: using stdlib smtp for now; can swap to github.com/wneessen/go-mail later.

func smtpAuth() (addr string, auth smtp.Auth, tlsConfig *tls.Config, useTLS bool, useStartTLS bool) {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM_EMAIL")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "25"
	}
	addr = fmt.Sprintf("%s:%s", host, port)
	if strings.TrimSpace(user) != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	} else {
		auth = nil
	}

	// TLS options
	useTLS = strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_USE_TLS"))) == "true"
	useStartTLS = strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_STARTTLS"))) == "true"
	skipVerify := strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_TLS_SKIP_VERIFY"))) == "true"

	tlsConfig = &tls.Config{InsecureSkipVerify: skipVerify, ServerName: host}
	_ = from
	return
}

// SendConfirmEmail sends confirmation email asynchronously.
func SendConfirmEmail(toEmail, toName, confirmLink string) error {
	data := map[string]interface{}{
		"Name":          toName,
		"ConfirmLink":   confirmLink,
		"ExpiryMinutes": os.Getenv("CONFIRM_TOKEN_TTL_MIN"),
	}

	m := &ConfirmMailable{
		subject:      "Confirm your account",
		templateBase: "templates/email/confirm",
		data:         data,
	}

	mailer := NewMailer()
	mailer.Queue(toEmail, m)
	return nil
}

// ConfirmMailable implements Mailable for confirmation emails.
type ConfirmMailable struct {
	subject      string
	templateBase string
	data         map[string]interface{}
}

func (c *ConfirmMailable) Subject() string {
	return c.subject
}

func (c *ConfirmMailable) TemplateBase() string {
	return c.templateBase
}

func (c *ConfirmMailable) Data() map[string]interface{} {
	return c.data
}

func (c *ConfirmMailable) From() (string, string) {
	return "", ""
}
