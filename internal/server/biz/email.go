package biz

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"time"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
)

//go:embed email/templates/*.html email/templates/*.txt
var templateFS embed.FS

type emailTemplateData struct {
	BrandName     string
	RecipientName string
	ActionURL     string
	Extra         string
}

// EmailServiceParams holds the dependencies for EmailService.
type EmailServiceParams struct {
	fx.In

	Ent            *ent.Client
	SystemService  *SystemService
}

// EmailService handles sending emails via SMTP with HTML/text templates.
type EmailService struct {
	db            *ent.Client
	systemService *SystemService
	htmlTemplates *template.Template
	textTemplates *template.Template
}

// NewEmailService creates a new EmailService.
func NewEmailService(params EmailServiceParams) *EmailService {
	htmlTmpl := template.Must(template.New("").ParseFS(templateFS, "email/templates/*.html"))
	textTmpl := template.Must(template.New("").ParseFS(templateFS, "email/templates/*.txt"))
	return &EmailService{
		db:            params.Ent,
		systemService: params.SystemService,
		htmlTemplates: htmlTmpl,
		textTemplates: textTmpl,
	}
}

func (s *EmailService) brandName(ctx context.Context) string {
	name, err := s.systemService.BrandName(ctx)
	if err != nil || name == "" {
		return "AxonHub"
	}
	return name
}

func (s *EmailService) renderTemplate(name string, data *emailTemplateData) (string, string, error) {
	var htmlBuf, textBuf bytes.Buffer
	if err := s.htmlTemplates.ExecuteTemplate(&htmlBuf, "email/templates/"+name+".html", data); err != nil {
		return "", "", fmt.Errorf("render html template %s: %w", name, err)
	}
	if err := s.textTemplates.ExecuteTemplate(&textBuf, "email/templates/"+name+".txt", data); err != nil {
		return "", "", fmt.Errorf("render text template %s: %w", name, err)
	}
	return htmlBuf.String(), textBuf.String(), nil
}

// Send sends an email with both HTML and plain text bodies via SMTP.
func (s *EmailService) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	settings, err := s.systemService.EmailSettings(ctx)
	if err != nil {
		return fmt.Errorf("get email settings: %w", err)
	}
	if settings.SMTPHost == "" {
		return fmt.Errorf("email service not configured")
	}

	from := settings.FromAddress
	if settings.FromName != "" {
		from = fmt.Sprintf("%s <%s>", settings.FromName, settings.FromAddress)
	}

	var msg bytes.Buffer
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"axonhub-email-boundary\"\r\n\r\n")
	msg.WriteString("--axonhub-email-boundary\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString(textBody + "\r\n\r\n")
	msg.WriteString("--axonhub-email-boundary\r\n")
	msg.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	msg.WriteString(htmlBody + "\r\n\r\n")
	msg.WriteString("--axonhub-email-boundary--")

	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)
	auth := smtp.PlainAuth("", settings.SMTPUser, settings.SMTPPassword, settings.SMTPHost)
	tlsConfig := &tls.Config{ServerName: settings.SMTPHost}
	if settings.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	switch settings.Encryption {
	case "ssl":
		return s.sendSSL(ctx, addr, tlsConfig, auth, settings.FromAddress, to, msg.Bytes())
	default:
		return s.sendDefault(ctx, addr, settings.SMTPHost, tlsConfig, auth, settings, settings.FromAddress, to, msg.Bytes())
	}
}

func (s *EmailService) sendSSL(ctx context.Context, addr string, tlsConfig *tls.Config, auth smtp.Auth, from, to string, msg []byte) error {
	dialer := &tls.Dialer{Config: tlsConfig}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, tlsConfig.ServerName)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return s.sendMail(c, from, to, msg)
}

func (s *EmailService) sendDefault(ctx context.Context, addr, host string, tlsConfig *tls.Config, auth smtp.Auth, settings *EmailSettings, from, to string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	if settings.Encryption == "starttls" {
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return s.sendMail(c, from, to, msg)
}

func (s *EmailService) sendMail(c *smtp.Client, from, to string, msg []byte) error {
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

// SendVerificationEmail sends an email verification link to the user.
func (s *EmailService) SendVerificationEmail(ctx context.Context, userEmail, userName, tokenURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     tokenURL,
	}
	htmlBody, textBody, err := s.renderTemplate("verify_email", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Verify Your Email", htmlBody, textBody)
}

// SendPasswordResetEmail sends a password reset link to the user.
func (s *EmailService) SendPasswordResetEmail(ctx context.Context, userEmail, userName, tokenURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     tokenURL,
	}
	htmlBody, textBody, err := s.renderTemplate("reset_password", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Reset Your Password", htmlBody, textBody)
}

// SendApprovedEmail notifies the user that their account has been approved.
func (s *EmailService) SendApprovedEmail(ctx context.Context, userEmail, userName, signInURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     signInURL,
	}
	htmlBody, textBody, err := s.renderTemplate("account_approved", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Account Approved", htmlBody, textBody)
}

// SendRejectedEmail notifies the user that their registration was not approved.
func (s *EmailService) SendRejectedEmail(ctx context.Context, userEmail, userName string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
	}
	htmlBody, textBody, err := s.renderTemplate("account_rejected", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Registration Update", htmlBody, textBody)
}

// SendAdminNotification notifies an admin about a new pending user.
func (s *EmailService) SendAdminNotification(ctx context.Context, adminEmail, userName, userEmail, reviewURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     reviewURL,
		Extra:         userEmail,
	}
	htmlBody, textBody, err := s.renderTemplate("admin_notification", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, adminEmail, s.brandName(ctx)+" — New User Requires Approval", htmlBody, textBody)
}

// SendTestEmail sends a simple test email to verify SMTP configuration.
func (s *EmailService) SendTestEmail(ctx context.Context, to string) error {
	brand := s.brandName(ctx)
	htmlBody := fmt.Sprintf(`<div style="font-family:sans-serif;padding:32px;"><h2>Email Test from %s</h2><p>If you're seeing this, your SMTP configuration is working correctly.</p></div>`, brand)
	textBody := fmt.Sprintf("Email Test from %s\n\nIf you're seeing this, your SMTP configuration is working correctly.", brand)
	return s.Send(ctx, to, brand+" — Test Email", htmlBody, textBody)
}

// TestConnection dials the SMTP server to verify connectivity without sending.
func (s *EmailService) TestConnection(ctx context.Context) error {
	settings, err := s.systemService.EmailSettings(ctx)
	if err != nil {
		return fmt.Errorf("get email settings: %w", err)
	}
	if settings.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConfig := &tls.Config{ServerName: settings.SMTPHost}
	if settings.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	switch settings.Encryption {
	case "ssl":
		tlsDialer := &tls.Dialer{
			Config: tlsConfig,
			NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		}
		conn, err := tlsDialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("connect failed: %w", err)
		}
		conn.Close()
	default:
		netConn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("connect failed: %w", err)
		}
		c, err := smtp.NewClient(netConn, settings.SMTPHost)
		if err != nil {
			netConn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		c.Close()
	}
	return nil
}
