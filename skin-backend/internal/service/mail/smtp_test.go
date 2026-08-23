package mail_test

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	mailsvc "element-skin/backend/internal/service/mail"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSMTPDeliversEmailChangeVerificationMessageExactly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	messages := make(chan string, 1)
	errors := make(chan error, 1)
	go serveOneSMTPConnection(listener, messages, errors)

	db, _ := testutil.NewTestApp(t)
	cache := redisstore.NewMemoryStore()
	host, portRaw, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{
		"smtp_host":     host,
		"smtp_port":     port,
		"smtp_user":     "smtp-user@test.com",
		"smtp_password": "smtp-password",
		"smtp_ssl":      false,
		"smtp_sender":   "Element Skin <no-reply@test.com>",
	} {
		if err := db.Settings.Set(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	sender := mailsvc.SMTP{Settings: settingssvc.Settings{DB: db, Redis: cache}}
	if err := sender.SendVerificationCode(t.Context(), "new-email@test.com", "EMAIL123", "email_change"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errors:
		t.Fatal(err)
	case message := <-messages:
		for _, expected := range []string{
			"From: \"Element Skin\" <no-reply@test.com>",
			"To: new-email@test.com",
			"Content-Type: text/html; charset=UTF-8",
			"<h2>重设邮箱</h2>",
			"EMAIL123",
			"设为 Element Skin 账号的新邮箱",
		} {
			if !strings.Contains(message, expected) {
				t.Fatalf("SMTP message missing %q: %q", expected, message)
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
}

func TestSMTPRejectsIncompleteSettingsAndUnknownPurposeExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := redisstore.NewMemoryStore()
	settings := settingssvc.Settings{DB: db, Redis: cache}
	for key, value := range map[string]any{
		"smtp_host":     "",
		"smtp_user":     "",
		"smtp_password": "",
	} {
		if err := db.Settings.Set(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	sender := mailsvc.SMTP{Settings: settings}
	if err := sender.SendVerificationCode(t.Context(), "new-email@test.com", "EMAIL123", "email_change"); err == nil || err.Error() != "SMTP settings are incomplete" {
		t.Fatalf("incomplete SMTP error=%v", err)
	}

	for key, value := range map[string]any{
		"smtp_host":     "127.0.0.1",
		"smtp_user":     "smtp-user@test.com",
		"smtp_password": "smtp-password",
	} {
		if err := db.Settings.Set(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := settings.InvalidateCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := sender.SendVerificationCode(t.Context(), "new-email@test.com", "EMAIL123", "unknown"); err == nil || err.Error() != `unsupported verification purpose "unknown"` {
		t.Fatalf("unknown purpose error=%v", err)
	}
}

func TestSMTPRejectsInvalidPortAndConnectionFailureExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := redisstore.NewMemoryStore()
	settings := settingssvc.Settings{DB: db, Redis: cache}
	for key, value := range map[string]any{
		"smtp_host":     "127.0.0.1",
		"smtp_port":     "invalid",
		"smtp_user":     "smtp-user@test.com",
		"smtp_password": "smtp-password",
		"smtp_ssl":      false,
		"smtp_sender":   "smtp-user@test.com",
	} {
		if err := db.Settings.Set(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	sender := mailsvc.SMTP{Settings: settings}
	if err := sender.SendVerificationCode(t.Context(), "recipient@test.com", "REGISTER1", "register"); err == nil || err.Error() != `invalid SMTP port "invalid"` {
		t.Fatalf("invalid SMTP port error=%v", err)
	}
	if err := db.Settings.Set(t.Context(), "smtp_port", 1); err != nil {
		t.Fatal(err)
	}
	if err := settings.InvalidateCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := sender.SendVerificationCode(t.Context(), "recipient@test.com", "REGISTER1", "register"); err == nil || !strings.HasPrefix(err.Error(), "connect SMTP server:") {
		t.Fatalf("SMTP connection error=%v", err)
	}
}

func TestSMTPUsesSenderFallbackAndExactCopyForRegisterAndReset(t *testing.T) {
	for _, tc := range []struct {
		purpose     string
		code        string
		wantHeading string
		wantText    string
	}{
		{purpose: "register", code: "REGISTER1", wantHeading: "欢迎注册 Element Skin", wantText: "验证注册邮箱"},
		{purpose: "reset", code: "RESET123", wantHeading: "重置密码", wantText: "重置 Element Skin 账号密码"},
	} {
		t.Run(tc.purpose, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			messages := make(chan string, 1)
			errorsFound := make(chan error, 1)
			go serveOneSMTPConnection(listener, messages, errorsFound)
			db, _ := testutil.NewTestApp(t)
			host, portRaw, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range map[string]any{
				"smtp_host": host, "smtp_port": portRaw, "smtp_user": "smtp-user@test.com",
				"smtp_password": "smtp-password", "smtp_ssl": false, "smtp_sender": "not an address <",
			} {
				if err := db.Settings.Set(t.Context(), key, value); err != nil {
					t.Fatal(err)
				}
			}
			sender := mailsvc.SMTP{Settings: settingssvc.Settings{DB: db, Redis: redisstore.NewMemoryStore()}}
			if err := sender.SendVerificationCode(t.Context(), "recipient@test.com", tc.code, tc.purpose); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-errorsFound:
				t.Fatal(err)
			case message := <-messages:
				for _, expected := range []string{"From: <smtp-user@test.com>", tc.wantHeading, tc.wantText, tc.code} {
					if !strings.Contains(message, expected) {
						t.Fatalf("%s message missing %q: %q", tc.purpose, expected, message)
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatal("timed out waiting for SMTP purpose message")
			}
		})
	}
}

func TestSMTPReportsExactProtocolStageFailures(t *testing.T) {
	for _, tc := range []struct {
		stage      string
		wantPrefix string
	}{
		{stage: "mail", wantPrefix: "set SMTP sender:"},
		{stage: "recipient", wantPrefix: "set SMTP recipient:"},
		{stage: "data", wantPrefix: "open SMTP message:"},
		{stage: "body", wantPrefix: "close SMTP message:"},
		{stage: "quit", wantPrefix: "finish SMTP delivery:"},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			serverErrors := make(chan error, 1)
			go serveSMTPFailure(listener, tc.stage, serverErrors)
			db, _ := testutil.NewTestApp(t)
			host, portRaw, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range map[string]any{
				"smtp_host": host, "smtp_port": portRaw, "smtp_user": "smtp-user@test.com",
				"smtp_password": "smtp-password", "smtp_ssl": false, "smtp_sender": "sender@test.com",
			} {
				if err := db.Settings.Set(t.Context(), key, value); err != nil {
					t.Fatal(err)
				}
			}
			sender := mailsvc.SMTP{Settings: settingssvc.Settings{DB: db, Redis: redisstore.NewMemoryStore()}}
			err = sender.SendVerificationCode(t.Context(), "recipient@test.com", "EMAIL123", "email_change")
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Fatalf("SMTP %s error=%v want prefix %q", tc.stage, err, tc.wantPrefix)
			}
			select {
			case serverErr := <-serverErrors:
				t.Fatal(serverErr)
			default:
			}
		})
	}
}

func serveOneSMTPConnection(listener net.Listener, messages chan<- string, errors chan<- error) {
	conn, err := listener.Accept()
	if err != nil {
		errors <- err
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("220 localhost ESMTP\r\n")); err != nil {
		errors <- err
		return
	}
	reader := bufio.NewReader(conn)
	var message strings.Builder
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			errors <- err
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if inData {
			if trimmed == "." {
				inData = false
				messages <- message.String()
				_, err = conn.Write([]byte("250 queued\r\n"))
				if err != nil {
					errors <- err
					return
				}
				continue
			}
			message.WriteString(line)
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "EHLO"):
			_, err = conn.Write([]byte("250-localhost\r\n250 OK\r\n"))
		case strings.HasPrefix(trimmed, "MAIL FROM:"):
			_, err = conn.Write([]byte("250 sender ok\r\n"))
		case strings.HasPrefix(trimmed, "RCPT TO:"):
			_, err = conn.Write([]byte("250 recipient ok\r\n"))
		case trimmed == "DATA":
			inData = true
			_, err = conn.Write([]byte("354 end with dot\r\n"))
		case trimmed == "QUIT":
			_, err = conn.Write([]byte("221 bye\r\n"))
			if err != nil {
				errors <- err
			}
			return
		default:
			_, err = conn.Write([]byte("250 ok\r\n"))
		}
		if err != nil {
			errors <- err
			return
		}
	}
}

func serveSMTPFailure(listener net.Listener, stage string, errors chan<- error) {
	conn, err := listener.Accept()
	if err != nil {
		errors <- err
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("220 localhost ESMTP\r\n")); err != nil {
		errors <- err
		return
	}
	reader := bufio.NewReader(conn)
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			errors <- err
			return
		}
		command := strings.TrimRight(line, "\r\n")
		if inData {
			if command != "." {
				continue
			}
			if stage == "body" {
				_, err = conn.Write([]byte("550 body rejected\r\n"))
				if err != nil {
					errors <- err
				}
				return
			}
			_, err = conn.Write([]byte("250 queued\r\n"))
			inData = false
		} else {
			switch {
			case strings.HasPrefix(command, "EHLO"):
				_, err = conn.Write([]byte("250 localhost\r\n"))
			case strings.HasPrefix(command, "MAIL FROM:"):
				if stage == "mail" {
					_, err = conn.Write([]byte("550 sender rejected\r\n"))
					if err == nil {
						return
					}
				} else {
					_, err = conn.Write([]byte("250 sender ok\r\n"))
				}
			case strings.HasPrefix(command, "RCPT TO:"):
				if stage == "recipient" {
					_, err = conn.Write([]byte("550 recipient rejected\r\n"))
					if err == nil {
						return
					}
				} else {
					_, err = conn.Write([]byte("250 recipient ok\r\n"))
				}
			case command == "DATA":
				if stage == "data" {
					_, err = conn.Write([]byte("550 data rejected\r\n"))
					if err == nil {
						return
					}
				} else {
					inData = true
					_, err = conn.Write([]byte("354 end with dot\r\n"))
				}
			case command == "QUIT":
				if stage == "quit" {
					_, err = conn.Write([]byte("550 quit rejected\r\n"))
				} else {
					_, err = conn.Write([]byte("221 bye\r\n"))
				}
				if err != nil {
					errors <- err
				}
				return
			default:
				_, err = conn.Write([]byte("250 ok\r\n"))
			}
		}
		if err != nil {
			errors <- err
			return
		}
	}
}
