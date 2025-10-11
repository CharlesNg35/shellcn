package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// ErrSMTPDisabled signals that SMTP delivery is disabled via configuration.
var ErrSMTPDisabled = errors.New("smtp: delivery disabled")

// Message represents an outbound email.
type Message struct {
	From    string
	To      []string
	Subject string
	Body    string
}

// Mailer defines behaviour for sending email messages.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPSettings capture the runtime configuration required by the SMTP mailer.
type SMTPSettings struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	UseTLS   bool
	Timeout  time.Duration
}

type smtpMailer struct {
	cfg    SMTPSettings
	dialFn smtpDialFunc
	authFn smtpAuthFunc
}

func (m *smtpMailer) Send(ctx context.Context, msg Message) error {
	if !m.cfg.Enabled {
		return ErrSMTPDisabled
	}

	recipients := uniqueAddresses(msg.To)
	if len(recipients) == 0 {
		return errors.New("smtp: at least one recipient is required")
	}

	from := strings.TrimSpace(msg.From)
	if from == "" {
		from = m.cfg.From
	}
	if from == "" {
		return errors.New("smtp: sender address is required")
	}

	if _, err := mail.ParseAddress(from); err != nil {
		return fmt.Errorf("smtp: invalid from address: %w", err)
	}

	for _, rcpt := range recipients {
		if _, err := mail.ParseAddress(rcpt); err != nil {
			return fmt.Errorf("smtp: invalid recipient address %q: %w", rcpt, err)
		}
	}

	conn, client, err := m.dialFn(ctx, m.cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer client.Close()

	if err := m.authFn(client, m.cfg); err != nil {
		return err
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp: rcpt to %s: %w", rcpt, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data command: %w", err)
	}

	if _, err := io.WriteString(wc, formatMessage(from, recipients, msg.Subject, msg.Body)); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp: write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp: close data writer: %w", err)
	}

	return client.Quit()
}

func validateSMTPConfig(cfg SMTPSettings) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("smtp: host is required when enabled")
	}
	if cfg.Port == 0 {
		return errors.New("smtp: port is required when enabled")
	}
	return nil
}

func uniqueAddresses(addresses []string) []string {
	seen := make(map[string]struct{}, len(addresses))
	var result []string
	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if _, exists := seen[addr]; exists {
			continue
		}
		seen[addr] = struct{}{}
		result = append(result, addr)
	}
	return result
}

type smtpClient interface {
	Mail(string) error
	Rcpt(string) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
	StartTLS(*tls.Config) error
	Auth(smtp.Auth) error
	Extension(string) (bool, string)
}

type smtpDialFunc func(ctx context.Context, cfg SMTPSettings) (net.Conn, smtpClient, error)
type smtpAuthFunc func(client smtpClient, cfg SMTPSettings) error

func NewSMTPMailer(cfg SMTPSettings) (Mailer, error) {
	if err := validateSMTPConfig(cfg); err != nil {
		return nil, err
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &smtpMailer{
		cfg:    cfg,
		dialFn: defaultDialFunc,
		authFn: defaultAuthFunc,
	}, nil
}

func defaultDialFunc(ctx context.Context, cfg SMTPSettings) (net.Conn, smtpClient, error) {
	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	dialer := &net.Dialer{Timeout: cfg.Timeout}

	var (
		conn net.Conn
		err  error
	)

	if cfg.UseTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: cfg.Host})
	} else if ctx != nil {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("smtp: dial %s: %w", address, err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("smtp: new client: %w", err)
	}

	if !cfg.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				_ = client.Close()
				_ = conn.Close()
				return nil, nil, fmt.Errorf("smtp: start tls: %w", err)
			}
		}
	}

	return conn, &realSMTPClient{Client: client}, nil
}

func defaultAuthFunc(client smtpClient, cfg SMTPSettings) error {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil
	}
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp: auth: %w", err)
	}
	return nil
}

type realSMTPClient struct {
	*smtp.Client
}

func formatMessage(from string, to []string, subject, body string) string {
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", strings.Join(to, ", ")),
		fmt.Sprintf("Subject: %s", escapeHeader(subject)),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
	}

	return strings.Join(headers, "\r\n") + body
}

func escapeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
