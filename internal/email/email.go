// Package email sends transactional mail (account invitations) over SMTP, using
// configuration loaded at startup. When disabled, callers fall back to sharing
// an invite's link directly.
package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// ErrNotConfigured is returned by Send when SMTP is disabled or incomplete.
var ErrNotConfigured = errors.New("email: SMTP not configured")

// SMTP is the outbound mail configuration.
type SMTP struct {
	Enabled  bool
	Host     string
	Port     int
	From     string
	Username string
	Password string
	UseTLS   bool
}

// Mailer sends mail through a configured SMTP server.
type Mailer struct {
	cfg SMTP
}

func New(cfg SMTP) *Mailer { return &Mailer{cfg: cfg} }

// Enabled reports whether a usable SMTP configuration is present.
func (m *Mailer) Enabled() bool {
	return m.cfg.Enabled && m.cfg.Host != "" && m.cfg.From != ""
}

// Send delivers a plain-text message to a single recipient.
func (m *Mailer) Send(to, subject, body string) error {
	if !m.Enabled() {
		return ErrNotConfigured
	}
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("email: invalid recipient or subject")
	}

	msg := []byte("From: " + m.cfg.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Date: " + time.Now().Format(time.RFC1123Z) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body + "\r\n")

	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	if m.cfg.UseTLS {
		return m.sendImplicitTLS(addr, auth, to, msg)
	}
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, msg)
}

// sendImplicitTLS dials a TLS socket first (e.g. port 465), where the server
// does not offer STARTTLS upgrade.
func (m *Mailer) sendImplicitTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.cfg.Host})
	if err != nil {
		return fmt.Errorf("email: dial: %w", err)
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: client: %w", err)
	}
	defer func() { _ = c.Close() }()

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
