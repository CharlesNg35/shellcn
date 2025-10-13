package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestNewSMTPMailerValidatesConfig(t *testing.T) {
	_, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
	})
	if err == nil || !strings.Contains(err.Error(), "host is required") {
		t.Fatalf("expected host validation error, got %v", err)
	}

	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("expected disabled configuration to succeed: %v", err)
	}

	if mailer == nil {
		t.Fatal("expected mailer to be returned")
	}
}

func TestSMTPMailerSendDisabled(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	err = mailer.Send(context.Background(), Message{
		To:      []string{"test@example.com"},
		Subject: "Test",
		Body:    "Hello",
	})
	if err != ErrSMTPDisabled {
		t.Fatalf("expected ErrSMTPDisabled, got %v", err)
	}
}

func TestFormatMessage(t *testing.T) {
	content := formatMessage("from@example.com", []string{"to@example.com"}, "Subject\r\nBreak", "Body")
	if !strings.Contains(content, "From: from@example.com") {
		t.Fatalf("expected from header, got %q", content)
	}
	if !strings.Contains(content, "Subject: Subject  Break") {
		t.Fatalf("expected sanitised subject, got %q", content)
	}
	if !strings.HasSuffix(content, "Body") {
		t.Fatalf("expected body suffix, got %q", content)
	}
}

func TestSMTPMailerDefaultTimeout(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "no-reply@example.com",
		UseTLS:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	sm, ok := mailer.(*smtpMailer)
	if !ok {
		t.Fatalf("expected smtpMailer type")
	}

	if sm.cfg.Timeout <= 0 {
		t.Fatalf("expected timeout to be assigned")
	}

	if sm.cfg.Timeout != 10*time.Second {
		t.Fatalf("expected timeout to be 10s, got %v", sm.cfg.Timeout)
	}
}

func TestSMTPMailerSendRequiresRecipients(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "no-reply@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	err = mailer.Send(context.Background(), Message{
		To:      []string{"   ", "\t"},
		Subject: "No recipients",
		Body:    "Body",
	})
	if err == nil || !strings.Contains(err.Error(), "at least one recipient") {
		t.Fatalf("expected missing recipient error, got %v", err)
	}
}

func TestSMTPMailerSendValidatesFromAddress(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "",
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	err = mailer.Send(context.Background(), Message{
		From: "invalid-from",
		To:   []string{"user@example.com"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid from address") {
		t.Fatalf("expected invalid from error, got %v", err)
	}
}

func TestSMTPMailerSendValidatesRecipientAddresses(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "no-reply@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	err = mailer.Send(context.Background(), Message{
		To: []string{"user@example.com", "bad-address"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid recipient address") {
		t.Fatalf("expected invalid recipient error, got %v", err)
	}
}

func TestUniqueAddresses(t *testing.T) {
	addresses := []string{"alice@example.com", "bob@example.com", " alice@example.com ", "", "bob@example.com"}
	result := uniqueAddresses(addresses)
	if len(result) != 2 {
		t.Fatalf("expected 2 unique addresses, got %d: %v", len(result), result)
	}
	if result[0] != "alice@example.com" || result[1] != "bob@example.com" {
		t.Fatalf("unexpected result order/content: %v", result)
	}
}

func TestSMTPMailerSendUsesDialAndAuth(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "no-reply@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	sm, ok := mailer.(*smtpMailer)
	if !ok {
		t.Fatalf("expected *smtpMailer, got %T", mailer)
	}

	fakeClient := &stubSMTPClient{startTLSSupported: false}
	sm.dialFn = func(ctx context.Context, cfg SMTPSettings) (net.Conn, smtpClient, error) {
		return nopConn{}, fakeClient, nil
	}

	var authCalled bool
	sm.authFn = func(client smtpClient, cfg SMTPSettings) error {
		authCalled = true
		return nil
	}

	err = mailer.Send(context.Background(), Message{
		To:      []string{"user@example.com"},
		Subject: "Subject",
		Body:    "Body",
	})
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}

	if !authCalled {
		t.Fatal("expected auth function to be called")
	}
	if fakeClient.mailFrom != "no-reply@example.com" {
		t.Fatalf("mail from = %q, want no-reply@example.com", fakeClient.mailFrom)
	}
	if len(fakeClient.rcptTo) != 1 || fakeClient.rcptTo[0] != "user@example.com" {
		t.Fatalf("unexpected rcpt list: %v", fakeClient.rcptTo)
	}
	if body := fakeClient.data.String(); !strings.Contains(body, "Body") {
		t.Fatalf("written body missing payload: %q", body)
	}
	if !fakeClient.quitCalled {
		t.Fatal("expected Quit to be called")
	}
}

func TestSMTPMailerSendAuthError(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPSettings{
		Enabled: true,
		Host:    "smtp.example.com",
		Port:    587,
		From:    "no-reply@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error creating mailer: %v", err)
	}

	sm := mailer.(*smtpMailer)
	sm.dialFn = func(ctx context.Context, cfg SMTPSettings) (net.Conn, smtpClient, error) {
		return nopConn{}, &stubSMTPClient{}, nil
	}
	sm.authFn = func(client smtpClient, cfg SMTPSettings) error {
		return errors.New("auth failure")
	}

	err = mailer.Send(context.Background(), Message{
		To: []string{"user@example.com"},
	})
	if err == nil || !strings.Contains(err.Error(), "auth failure") {
		t.Fatalf("expected auth failure error, got %v", err)
	}
}

func TestDefaultAuthFunc(t *testing.T) {
	client := &stubSMTPClient{}
	cfg := SMTPSettings{
		Username: "user",
		Password: "pass",
		Host:     "smtp.example.com",
	}

	if err := defaultAuthFunc(client, cfg); err != nil {
		t.Fatalf("defaultAuthFunc returned error: %v", err)
	}
	if !client.authCalled {
		t.Fatal("expected Auth to be invoked")
	}

	cfg.Username = ""
	if err := defaultAuthFunc(client, cfg); err != nil {
		t.Fatalf("expected nil error when username empty, got %v", err)
	}
}

type nopConn struct{}

func (nopConn) Read(b []byte) (int, error)       { return 0, io.EOF }
func (nopConn) Write(b []byte) (int, error)      { return len(b), nil }
func (nopConn) Close() error                     { return nil }
func (nopConn) LocalAddr() net.Addr              { return dummyAddr("local") }
func (nopConn) RemoteAddr() net.Addr             { return dummyAddr("remote") }
func (nopConn) SetDeadline(time.Time) error      { return nil }
func (nopConn) SetReadDeadline(time.Time) error  { return nil }
func (nopConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return string(a) }
func (a dummyAddr) String() string  { return string(a) }

type stubSMTPClient struct {
	mailFrom          string
	rcptTo            []string
	data              bytes.Buffer
	quitCalled        bool
	authCalled        bool
	startTLSSupported bool
}

func (s *stubSMTPClient) Mail(from string) error {
	s.mailFrom = from
	return nil
}

func (s *stubSMTPClient) Rcpt(to string) error {
	s.rcptTo = append(s.rcptTo, to)
	return nil
}

func (s *stubSMTPClient) Data() (io.WriteCloser, error) {
	return nopWriteCloser{Writer: &s.data}, nil
}

func (s *stubSMTPClient) Quit() error {
	s.quitCalled = true
	return nil
}

func (s *stubSMTPClient) Close() error { return nil }

func (s *stubSMTPClient) StartTLS(*tls.Config) error {
	s.startTLSSupported = true
	return nil
}

func (s *stubSMTPClient) Auth(smtp.Auth) error {
	s.authCalled = true
	return nil
}

func (s *stubSMTPClient) Extension(name string) (bool, string) {
	if strings.EqualFold(name, "STARTTLS") {
		return s.startTLSSupported, ""
	}
	return false, ""
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }
