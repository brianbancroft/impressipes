package server

import (
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/brianbancroft/impressipies/apps/api/internal/apple"
	"log"
	"strings"

	"github.com/brianbancroft/impressipies/apps/api/internal/mail"
)

type options struct {
	mailer    mail.Mailer
	to        string
	baseURL   string
	appleTeam string

	passkeys *webauthn.Config
	apple    *apple.Verifier
	appleWeb string // Services ID for the web button
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

// WithAppleTeam serves /.well-known/apple-app-site-association so the emailed
// sign-in link opens the iOS app. team is the Apple Developer Team ID.
func WithAppleTeam(team string) Option {
	return func(o *options) { o.appleTeam = team }
}

// WithPasskeys turns on passkey sign-in. rpID is the site's domain (such as
// "impressipes.bancroft.io"); origins are the full origins that may sign in, which
// must include the web app's.
func WithPasskeys(rpID string, origins ...string) Option {
	return func(o *options) {
		o.passkeys = &webauthn.Config{RPID: rpID, RPDisplayName: "Impressipes", RPOrigins: origins}
	}
}

// WithApple turns on Sign in with Apple. v holds the accepted audiences (the iOS
// bundle ID and, for the web button, the Services ID); webClientID is that Services
// ID, which the web app needs to start the flow, or empty to hide the web button.
func WithApple(v *apple.Verifier, webClientID string) Option {
	return func(o *options) { o.apple, o.appleWeb = v, webClientID }
}
