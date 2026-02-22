package mail

import (
	"bufio"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP starts a minimal plain SMTP server that accepts one message.
func startFakeSMTP(t *testing.T, addr string) (stop func()) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				close(done)
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				// greet
				c.Write([]byte("220 localhost ESMTP\r\n"))
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimSpace(line)
					if strings.HasPrefix(strings.ToUpper(line), "EHLO") || strings.HasPrefix(strings.ToUpper(line), "HELO") {
						c.Write([]byte("250-localhost Hello\r\n250 AUTH PLAIN\r\n"))
						continue
					}
					if strings.HasPrefix(strings.ToUpper(line), "MAIL FROM") {
						c.Write([]byte("250 OK\r\n"))
						continue
					}
					if strings.HasPrefix(strings.ToUpper(line), "RCPT TO") {
						c.Write([]byte("250 OK\r\n"))
						continue
					}
					if strings.HasPrefix(strings.ToUpper(line), "DATA") {
						c.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
						// read until single dot line
						for {
							l, err := r.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimSpace(l) == "." {
								break
							}
						}
						c.Write([]byte("250 OK: queued\r\n"))
						continue
					}
					if strings.HasPrefix(strings.ToUpper(line), "QUIT") {
						c.Write([]byte("221 Bye\r\n"))
						return
					}
				}
			}(conn)
		}
	}()
	return func() {
		ln.Close()
		<-done
	}
}

func TestRenderParts(t *testing.T) {
	data := map[string]interface{}{
		"Name":          "Tester",
		"ConfirmLink":   "https://example.com/confirm?token=abc",
		"ExpiryMinutes": "60",
	}
	m := NewMailer()
	html, txt, err := m.renderParts("templates/email/confirm", data)
	if err != nil {
		t.Fatalf("renderParts error: %v", err)
	}
	hs := string(html)
	ts := string(txt)
	// ensure confirm link is present in both outputs
	if !strings.Contains(hs, "https://example.com/confirm?token=abc") {
		t.Fatalf("html output missing confirm link: %s", hs)
	}
	if !strings.Contains(ts, "https://example.com/confirm?token=abc") {
		t.Fatalf("text output missing confirm link: %s", ts)
	}
}

func TestMailerSendWithFakeSMTP(t *testing.T) {
	// pick a high port to avoid clashes
	addr := "127.0.0.1:2526"
	stop := startFakeSMTP(t, addr)
	defer stop()

	// set env for smtpAuth
	hostPort := strings.Split(addr, ":")
	os.Setenv("SMTP_HOST", hostPort[0])
	os.Setenv("SMTP_PORT", hostPort[1])
	os.Setenv("SMTP_USER", "")
	os.Setenv("SMTP_PASS", "")
	os.Setenv("SMTP_FROM_EMAIL", "no-reply@example.com")
	os.Setenv("SMTP_FROM_NAME", "Test")

	// small pause to ensure listener ready
	time.Sleep(50 * time.Millisecond)

	data := map[string]interface{}{
		"Name":          "Tester",
		"ConfirmLink":   "https://example.com/confirm?token=abc",
		"ExpiryMinutes": "60",
	}
	m := NewMailer()
	mail := &ConfirmMailable{subject: "Confirm your account", templateBase: "templates/email/confirm", data: data}
	if err := m.Send("recipient@example.com", mail); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}
