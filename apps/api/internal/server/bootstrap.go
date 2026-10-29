package server

import (
	"log"
	"strings"

	"github.com/brianbancroft/impressipies/apps/api/internal/mail"
)

type options struct {
	mailer  mail.Mailer
	to      string
	baseURL string
}

// Option configures New.
type Option func(*options)

// WithBootstrapMail emails the first-run admin invite to `to`. baseURL is the
// public origin the invite path is appended to.
func WithBootstrapMail(m mail.Mailer, to, baseURL string) Option {
	return func(o *options) { o.mailer, o.to, o.baseURL = m, to, strings.TrimRight(baseURL, "/") }
}

func (o options) mailBootstrap(path string) {
	if o.mailer == nil || o.to == "" {
		return
	}
	link := o.baseURL + path
	body := "You've been invited to set up Impressipies as its first admin.\r\n\r\n" +
		"Open this link to create your account (valid for 7 days):\r\n" + link + "\r\n"
	if err := o.mailer.Send(o.to, "Your Impressipies admin invite", body); err != nil {
		log.Printf("bootstrap invite email to %s failed: %v", o.to, err)
		return
	}
	log.Printf("bootstrap invite emailed to %s", o.to)
}
