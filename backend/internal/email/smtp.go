package email

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
)

// SMTPSender sends email via an SMTP relay (e.g. Mailpit for local
// development, or AWS SES SMTP).
type SMTPSender struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSMTPSender returns a Sender that sends via the given SMTP server.
// If username is empty, no SMTP authentication is attempted (suitable for
// local dev servers such as Mailpit).
func NewSMTPSender(host, port, username, password, from string) *SMTPSender {
	return &SMTPSender{host: host, port: port, username: username, password: password, from: from}
}

// SendMagicLink implements Sender.
func (m *SMTPSender) SendMagicLink(_ context.Context, toEmail, link string) error {
	text, _ := magicLinkBody(link)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		m.from, toEmail, magicLinkSubject, text,
	)

	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	addr := net.JoinHostPort(m.host, m.port)
	return smtp.SendMail(addr, auth, m.from, []string{toEmail}, []byte(msg))
}
