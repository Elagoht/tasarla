package mail_test

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	kmail "kanban/internal/mail"
)

func TestBuildIsMultipartAlternative(t *testing.T) {
	raw, err := kmail.Build("Kanban <noreply@example.com>", "ada@example.com", "Kart atandı: Ödeme", "plain body", "<p>html body</p>", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	dec := new(mime.WordDecoder)
	subject, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	if subject != "Kart atandı: Ödeme" || msg.Header.Get("To") != "ada@example.com" || msg.Header.Get("Message-Id") == "" {
		t.Fatalf("headers = %v (subject %q)", msg.Header, subject)
	}
	mediaType, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if mediaType != "multipart/alternative" {
		t.Fatalf("type = %s", mediaType)
	}
	r := multipart.NewReader(msg.Body, params["boundary"])
	var parts []string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(p)
		parts = append(parts, p.Header.Get("Content-Type")+"|"+string(body))
	}
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "text/plain") || !strings.Contains(parts[0], "plain body") ||
		!strings.HasPrefix(parts[1], "text/html") || !strings.Contains(parts[1], "<p>html body</p>") {
		t.Fatalf("parts = %q", parts)
	}
}

// fakeSMTP accepts one message without TLS or auth and hands back its DATA.
func fakeSMTP(t *testing.T) (addr string, got <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer ln.Close()
		r := bufio.NewReader(conn)
		w := func(s string) { conn.Write([]byte(s + "\r\n")) }
		w("220 fake ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					w("250 queued")
					ch <- data.String()
					continue
				}
				data.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				w("250 fake")
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				w("354 go ahead")
			case strings.HasPrefix(cmd, "QUIT"):
				w("221 bye")
				return
			default:
				w("250 ok")
			}
		}
	}()
	return ln.Addr().String(), ch
}

func TestSenderTalksSMTP(t *testing.T) {
	addr, got := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	s := kmail.Sender{Host: host, Port: port, From: "noreply@example.com"}
	if err := s.Send(context.Background(), "ada@example.com", "Hello", "text", "<p>html</p>"); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-got:
		if !strings.Contains(data, "Subject: Hello") || !strings.Contains(data, "To: ada@example.com") {
			t.Fatalf("data = %s", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no message arrived")
	}
}

func TestSenderReportsAServerThatIsDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	host, port, _ := net.SplitHostPort(addr)
	s := kmail.Sender{Host: host, Port: port, From: "noreply@example.com", Timeout: time.Second}
	if err := s.Send(context.Background(), "ada@example.com", "x", "x", "x"); err == nil {
		t.Fatal("sending to a closed port succeeded")
	}
}
