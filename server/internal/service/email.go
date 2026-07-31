package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
)

const (
	emailProviderConsole = "console"
	emailProviderResend  = "resend"
	emailProviderSMTP    = "smtp"

	smtpTLSStartTLS = "starttls"
	smtpTLSImplicit = "tls"
	smtpTLSNone     = "none"
)

type emailConfig struct {
	provider     string
	from         *mail.Address
	resendAPIKey string
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	smtpTLSMode  string
	smtpTimeout  time.Duration
}

type EmailService struct {
	config       emailConfig
	resendClient *resend.Client
	configErr    error
}

func NewEmailService() *EmailService {
	config, err := newEmailConfig(os.Getenv)
	service := &EmailService{config: config, configErr: err}
	if err == nil && config.provider == emailProviderResend {
		service.resendClient = resend.NewClient(config.resendAPIKey)
	}
	return service
}

func newEmailConfig(getenv func(string) string) (emailConfig, error) {
	provider := strings.ToLower(strings.TrimSpace(getenv("EMAIL_PROVIDER")))
	resendAPIKey := strings.TrimSpace(getenv("RESEND_API_KEY"))
	smtpHost := strings.TrimSpace(getenv("SMTP_HOST"))

	if provider == "" {
		switch {
		case resendAPIKey != "" && smtpHost != "":
			return emailConfig{}, fmt.Errorf("EMAIL_PROVIDER is required when both Resend and SMTP are configured")
		case resendAPIKey != "":
			provider = emailProviderResend
		case smtpHost != "":
			provider = emailProviderSMTP
		default:
			provider = emailProviderConsole
		}
	}

	if provider == emailProviderConsole && strings.EqualFold(strings.TrimSpace(getenv("APP_ENV")), "production") {
		return emailConfig{}, fmt.Errorf("EMAIL_PROVIDER must be resend or smtp in production")
	}

	fromValue := strings.TrimSpace(getenv("EMAIL_FROM"))
	if fromValue == "" {
		fromValue = "Agentra <noreply@agentra.ai>"
	}
	from, err := mail.ParseAddress(fromValue)
	if err != nil || !strings.Contains(from.Address, "@") {
		return emailConfig{}, fmt.Errorf("EMAIL_FROM must be a valid email address")
	}

	config := emailConfig{
		provider:     provider,
		from:         from,
		resendAPIKey: resendAPIKey,
	}

	switch provider {
	case emailProviderConsole:
		return config, nil
	case emailProviderResend:
		if resendAPIKey == "" {
			return emailConfig{}, fmt.Errorf("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
		}
		return config, nil
	case emailProviderSMTP:
		return configureSMTP(config, getenv)
	default:
		return emailConfig{}, fmt.Errorf("EMAIL_PROVIDER must be console, resend, or smtp")
	}
}

func configureSMTP(config emailConfig, getenv func(string) string) (emailConfig, error) {
	config.smtpHost = strings.TrimSpace(getenv("SMTP_HOST"))
	if config.smtpHost == "" {
		return emailConfig{}, fmt.Errorf("SMTP_HOST is required when EMAIL_PROVIDER=smtp")
	}

	portValue := strings.TrimSpace(getenv("SMTP_PORT"))
	if portValue == "" {
		portValue = "587"
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return emailConfig{}, fmt.Errorf("SMTP_PORT must be an integer between 1 and 65535")
	}
	config.smtpPort = port

	config.smtpTLSMode = strings.ToLower(strings.TrimSpace(getenv("SMTP_TLS_MODE")))
	if config.smtpTLSMode == "" {
		config.smtpTLSMode = smtpTLSStartTLS
	}
	if config.smtpTLSMode != smtpTLSStartTLS && config.smtpTLSMode != smtpTLSImplicit && config.smtpTLSMode != smtpTLSNone {
		return emailConfig{}, fmt.Errorf("SMTP_TLS_MODE must be starttls, tls, or none")
	}

	config.smtpUsername = strings.TrimSpace(getenv("SMTP_USERNAME"))
	config.smtpPassword = getenv("SMTP_PASSWORD")
	if (config.smtpUsername == "") != (config.smtpPassword == "") {
		return emailConfig{}, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	if config.smtpUsername != "" && config.smtpTLSMode == smtpTLSNone {
		return emailConfig{}, fmt.Errorf("SMTP authentication requires TLS")
	}

	timeoutValue := strings.TrimSpace(getenv("SMTP_TIMEOUT"))
	if timeoutValue == "" {
		timeoutValue = "10s"
	}
	config.smtpTimeout, err = time.ParseDuration(timeoutValue)
	if err != nil || config.smtpTimeout <= 0 {
		return emailConfig{}, fmt.Errorf("SMTP_TIMEOUT must be a positive duration")
	}

	return config, nil
}

func (s *EmailService) SendVerificationCode(ctx context.Context, to, code string) (*string, error) {
	if s.configErr != nil {
		return nil, s.configErr
	}

	recipient, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil || !strings.Contains(recipient.Address, "@") {
		return nil, fmt.Errorf("recipient must be a valid email address")
	}

	subject := "Your Agentra verification code"
	textBody := fmt.Sprintf("Your Agentra verification code is %s. It expires in 10 minutes. If you did not request this code, you can ignore this email.", code)
	htmlBody := fmt.Sprintf(
		`<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
			<h2>Your verification code</h2>
			<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">%s</p>
			<p>This code expires in 10 minutes.</p>
			<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
		</div>`, html.EscapeString(code))

	switch s.config.provider {
	case emailProviderConsole:
		return &code, nil
	case emailProviderResend:
		_, err = s.resendClient.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
			From:    s.config.from.String(),
			To:      []string{recipient.Address},
			Subject: subject,
			Html:    htmlBody,
			Text:    textBody,
		})
	case emailProviderSMTP:
		err = s.sendSMTP(ctx, recipient, subject, textBody, htmlBody)
	default:
		err = fmt.Errorf("email provider is not configured")
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *EmailService) sendSMTP(ctx context.Context, to *mail.Address, subject, textBody, htmlBody string) error {
	message, err := buildVerificationMessage(s.config.from, to, subject, textBody, htmlBody)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.config.smtpHost, strconv.Itoa(s.config.smtpPort))
	dialer := &net.Dialer{Timeout: s.config.smtpTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}

	deadline := time.Now().Add(s.config.smtpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return fmt.Errorf("set SMTP deadline: %w", err)
	}

	tlsConfig := &tls.Config{ServerName: s.config.smtpHost, MinVersion: tls.VersionTLS12}
	if s.config.smtpTLSMode == smtpTLSImplicit {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return fmt.Errorf("negotiate SMTP TLS: %w", err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, s.config.smtpHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()

	if s.config.smtpTLSMode == smtpTLSStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not advertise STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}

	if s.config.smtpUsername != "" {
		auth := smtp.PlainAuth("", s.config.smtpUsername, s.config.smtpPassword, s.config.smtpHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(s.config.from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message body: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message body: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP session: %w", err)
	}
	return nil
}

func buildVerificationMessage(from, to *mail.Address, subject, textBody, htmlBody string) ([]byte, error) {
	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)

	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	textPart, err := multipartWriter.CreatePart(textHeader)
	if err != nil {
		return nil, fmt.Errorf("create email text part: %w", err)
	}
	if _, err := textPart.Write([]byte(textBody)); err != nil {
		return nil, fmt.Errorf("write email text part: %w", err)
	}

	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "8bit")
	htmlPart, err := multipartWriter.CreatePart(htmlHeader)
	if err != nil {
		return nil, fmt.Errorf("create email HTML part: %w", err)
	}
	if _, err := htmlPart.Write([]byte(htmlBody)); err != nil {
		return nil, fmt.Errorf("write email HTML part: %w", err)
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("close email body: %w", err)
	}

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	fmt.Fprint(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/alternative; boundary=%q\r\n", multipartWriter.Boundary())
	fmt.Fprint(&message, "\r\n")
	message.Write(body.Bytes())

	return message.Bytes(), nil
}
