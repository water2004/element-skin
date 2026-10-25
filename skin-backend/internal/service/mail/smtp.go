package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"

	settingssvc "element-skin/backend/internal/service/settings"
)

type Sender interface {
	SendVerificationCode(context.Context, string, string, string) error
}

type SMTP struct {
	Settings settingssvc.Settings
}

type smtpConfig struct {
	host        string
	port        int
	username    string
	password    string
	sender      mail.Address
	implicitTLS bool
}

func (s SMTP) SendVerificationCode(ctx context.Context, recipient, code, purpose string) error {
	cfg, err := s.config(ctx)
	if err != nil {
		return err
	}
	subject, heading, description, err := verificationCopy(purpose)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`<html><body><h2>%s</h2><p>%s</p><p>您的验证码是：<strong style="font-size:20px;color:#409eff">%s</strong></p><p>该验证码将在几分钟后过期。如果这不是您本人的操作，请忽略此邮件。</p></body></html>`, heading, description, code)
	message := []byte(strings.Join([]string{
		"From: " + cfg.sender.String(),
		"To: " + recipient,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n"))
	return send(ctx, cfg, recipient, message)
}

func (s SMTP) config(ctx context.Context) (smtpConfig, error) {
	host, err := s.Settings.Get(ctx, "smtp_host", "")
	if err != nil {
		return smtpConfig{}, err
	}
	portRaw, err := s.Settings.Get(ctx, "smtp_port", "465")
	if err != nil {
		return smtpConfig{}, err
	}
	username, err := s.Settings.Get(ctx, "smtp_user", "")
	if err != nil {
		return smtpConfig{}, err
	}
	password, err := s.Settings.Get(ctx, "smtp_password", "")
	if err != nil {
		return smtpConfig{}, err
	}
	senderRaw, err := s.Settings.Get(ctx, "smtp_sender", "")
	if err != nil {
		return smtpConfig{}, err
	}
	sslEnabled, err := s.Settings.Bool(ctx, "smtp_ssl", true)
	if err != nil {
		return smtpConfig{}, err
	}
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	if host == "" || username == "" || password == "" {
		return smtpConfig{}, fmt.Errorf("SMTP settings are incomplete")
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return smtpConfig{}, fmt.Errorf("invalid SMTP port %q", portRaw)
	}
	sender, err := mail.ParseAddress(strings.TrimSpace(senderRaw))
	if err != nil || sender.Address == "" {
		sender = &mail.Address{Address: username}
	}
	return smtpConfig{
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		sender:      *sender,
		implicitTLS: sslEnabled,
	}, nil
}

func verificationCopy(purpose string) (string, string, string, error) {
	switch purpose {
	case "register":
		return "Element Skin 验证码", "欢迎注册 Element Skin", "您正在验证注册邮箱。", nil
	case "reset":
		return "Element Skin 密码重置验证码", "重置密码", "您正在重置 Element Skin 账号密码。", nil
	case "email_change":
		return "Element Skin 邮箱重设验证码", "重设邮箱", "您正在将此邮箱设为 Element Skin 账号的新邮箱。", nil
	default:
		return "", "", "", fmt.Errorf("unsupported verification purpose %q", purpose)
	}
}

func send(ctx context.Context, cfg smtpConfig, recipient string, message []byte) error {
	address := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	var conn net.Conn
	var err error
	if cfg.implicitTLS {
		conn, err = (&tls.Dialer{Config: &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()
	if !cfg.implicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("start SMTP TLS: %w", err)
			}
		}
	}
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(smtp.PlainAuth("", cfg.username, cfg.password, cfg.host)); err != nil {
			return fmt.Errorf("authenticate SMTP client: %w", err)
		}
	}
	if err := client.Mail(cfg.sender.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		_ = w.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil && err != io.EOF {
		return fmt.Errorf("finish SMTP delivery: %w", err)
	}
	return nil
}
