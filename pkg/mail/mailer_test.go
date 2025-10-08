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
