package mail

import (
	"context"
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
