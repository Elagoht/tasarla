// Package mail builds and sends the application's e-mails over SMTP.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// Build writes an RFC 5322 message with a plain-text and an HTML part.
func Build(from, to, subject, text, html string, date time.Time) ([]byte, error) {
	var buf bytes.Buffer
	boundary := randomHex(12)
	host := "kanban"
	if _, domain, ok := strings.Cut(addressOf(from), "@"); ok {
		host = domain
	}
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + date.Format(time.RFC1123Z),
		"Message-ID: <" + randomHex(16) + "@" + host + ">",
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="` + boundary + `"`,
	}
	buf.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")
	w := multipart.NewWriter(&buf)
	if err := w.SetBoundary(boundary); err != nil {
		return nil, err
	}
	for _, part := range []struct{ kind, body string }{{"text/plain", text}, {"text/html", html}} {
		pw, err := w.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.kind + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Sender sends through one SMTP server. It uses STARTTLS when the server offers
// it, and authenticates only over TLS or to localhost.
type Sender struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	Timeout  time.Duration // default 30s
}

// Send delivers one message.
func (s Sender) Send(ctx context.Context, to, subject, text, html string) error {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	msg, err := Build(s.From, to, subject, text, html, time.Now())
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return fmt.Errorf("mail: dial: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: greeting: %w", err)
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(addressOf(s.From)); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: end data: %w", err)
	}
	return c.Quit()
}

// addressOf returns the bare address of "Name <addr>" or addr.
func addressOf(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		return strings.TrimSuffix(from[i+1:], ">")
	}
	return from
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
