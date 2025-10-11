package app

import "github.com/charlesng35/shellcn/pkg/mail"

// SMTPSettings converts EmailConfig to the mail package representation.
func (c EmailConfig) SMTPSettings() mail.SMTPSettings {
	return mail.SMTPSettings{
		Enabled:  c.SMTP.Enabled,
		Host:     c.SMTP.Host,
		Port:     c.SMTP.Port,
		Username: c.SMTP.Username,
		Password: c.SMTP.Password,
		From:     c.SMTP.From,
		UseTLS:   c.SMTP.UseTLS,
		Timeout:  c.SMTP.Timeout,
	}
}
