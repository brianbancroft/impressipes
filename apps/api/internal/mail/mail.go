// Package mail sends plain-text email over SMTP. Local development points it at
// Mailpit (localhost:1025), which needs no auth or TLS.
package mail

import (
	"fmt"
	stdmail "net/mail"
	"net/smtp"
	"strings"
)

type Mailer interface {
	Send(to, subject, body string) error
}

type SMTP struct {
	Addr string // host:port
	From string
}

func (m SMTP) Send(to, subject, body string) error {
	if strings.ContainsAny(to+subject+m.From, "\r\n") {
		return fmt.Errorf("mail: header injection")
	}
	from, err := stdmail.ParseAddress(m.From)
	if err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	msg := "From: " + m.From + "\r\nTo: " + to + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body
	return smtp.SendMail(m.Addr, nil, from.Address, []string{to}, []byte(msg))
}
