package mailer

import (
	"fmt"
	"net/smtp"

	"github.com/PauloHFS/goth/internal/config"
)

type Mailer struct {
	addr string
	auth smtp.Auth
	from string
}

func New(cfg *config.Config) *Mailer {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}

	return &Mailer{
		addr: addr,
		auth: auth,
		from: cfg.SMTPFrom,
	}
}

func (m *Mailer) Send(to, subject, body string) error {
	header := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", to, subject)
	msg := []byte(header + body)

	return smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg)
}
