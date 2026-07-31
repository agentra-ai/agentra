package service

import (
	"context"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestEmailConfigDefaultsToConsoleOutsideProduction(t *testing.T) {
	config, err := newEmailConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("newEmailConfig() error = %v", err)
	}
	if config.provider != emailProviderConsole {
		t.Fatalf("provider = %q, want %q", config.provider, emailProviderConsole)
	}
}

func TestEmailServiceDoesNotExposeCodeInProduction(t *testing.T) {
	service := &EmailService{configErr: newProductionConsoleConfigError(t)}
	devCode, err := service.SendVerificationCode(context.Background(), "person@example.com", "123456")
	if err == nil {
		t.Fatal("SendVerificationCode() expected a configuration error")
	}
	if devCode != nil {
		t.Fatal("SendVerificationCode() must not expose a development code in production")
	}
}

func newProductionConsoleConfigError(t *testing.T) error {
	t.Helper()
	_, err := newEmailConfig(func(key string) string {
		if key == "APP_ENV" {
			return "production"
		}
		return ""
	})
	if err == nil {
		t.Fatal("newEmailConfig() expected an error")
	}
	return err
}

func TestEmailConfigSupportsSMTP(t *testing.T) {
	values := map[string]string{
		"EMAIL_PROVIDER": "smtp",
		"EMAIL_FROM":     "Agentra <login@example.com>",
		"SMTP_HOST":      "smtp.example.com",
		"SMTP_PORT":      "465",
		"SMTP_USERNAME":  "agentra",
		"SMTP_PASSWORD":  "secret",
		"SMTP_TLS_MODE":  "tls",
		"SMTP_TIMEOUT":   "15s",
	}
	config, err := newEmailConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("newEmailConfig() error = %v", err)
	}
	if config.provider != emailProviderSMTP || config.smtpPort != 465 || config.smtpTimeout != 15*time.Second {
		t.Fatalf("unexpected SMTP config: %+v", config)
	}
}

func TestEmailConfigRejectsUnsafeOrIncompleteSMTP(t *testing.T) {
	tests := []map[string]string{
		{"EMAIL_PROVIDER": "smtp"},
		{"EMAIL_PROVIDER": "smtp", "SMTP_HOST": "smtp.example.com", "SMTP_PORT": "invalid"},
		{"EMAIL_PROVIDER": "smtp", "SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "user"},
		{"EMAIL_PROVIDER": "smtp", "SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "user", "SMTP_PASSWORD": "secret", "SMTP_TLS_MODE": "none"},
	}
	for _, values := range tests {
		if _, err := newEmailConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("newEmailConfig(%v) expected an error", values)
		}
	}
}

func TestBuildVerificationMessageIncludesTextAndHTML(t *testing.T) {
	from := &mail.Address{Name: "Agentra", Address: "login@example.com"}
	to := &mail.Address{Address: "person@example.com"}
	message, err := buildVerificationMessage(from, to, "Login code", "text body", "<strong>HTML body</strong>")
	if err != nil {
		t.Fatalf("buildVerificationMessage() error = %v", err)
	}
	contents := string(message)
	for _, expected := range []string{
		"From: ",
		"login@example.com>\r\n",
		"To: <person@example.com>\r\n",
		"Content-Type: multipart/alternative",
		"text/plain; charset=UTF-8",
		"text/html; charset=UTF-8",
		"text body",
		"<strong>HTML body</strong>",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("message does not contain %q", expected)
		}
	}
}
