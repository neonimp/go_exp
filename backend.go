package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/emersion/go-smtp"
)

type Backend struct {
	Cfg   *Config
	Queue *MailQueue
}

type Mail struct {
	From     string
	To       []string
	Headers  map[string]string
	Body     string
	MailData []byte
}

type Session struct {
	Cfg      *Config
	Queue    *MailQueue
	IsAuthed bool
	AuthUser string
	Current  Mail
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	s := &Session{
		Cfg:      b.Cfg,
		IsAuthed: false,
		AuthUser: "",
		Current:  Mail{},
		Queue:    b.Queue,
	}
	if b.Cfg.Auth.AllowAnon {
		log.Println("Allowing anonymous SMTP")
		s.IsAuthed = true
	}
	return s, nil
}

func (s *Session) AuthPlain(username string, password string) error {
	log.Println("AuthPlain:", username)
	if s.Cfg.Auth.AuthUsers == nil {
		log.Println("No auth users")
		return errors.New("no auth users configured")
	}
	for _, u := range s.Cfg.Auth.AuthUsers {
		e := strings.Split(u, ":")
		if len(e) != 2 {
			continue
		}
		if e[0] == username && e[1] == password {
			s.IsAuthed = true
			s.AuthUser = username
			return nil
		}
	}
	log.Println("Invalid username or password for SMTP authentication")
	return errors.New("invalid username or password for SMTP authentication")
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if !s.IsAuthed {
		log.Println("Not authenticated")
		return errors.New("not authenticated")
	}
	nMail := Mail{
		From: from,
	}
	nMail.From = from
	s.Current = nMail
	log.Println("Mail from:", from)
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if !s.IsAuthed {
		log.Println("Not authenticated")
		return errors.New("not authenticated")
	}
	log.Println("Rcpt to:", to)
	s.Current.To = append(s.Current.To, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	if !s.IsAuthed {
		log.Println("Not authenticated")
		return nil
	}
	if b, err := io.ReadAll(r); err != nil {
		return err
	} else {
		log.Println("Data size:", len(b))
		s.Current.MailData = b
	}
	h := parseHeaders(s.Current.MailData)
	b := parseBody(s.Current.MailData)
	s.Current.Headers = h
	s.Current.Body = b
	s.EnqueueMail(&s.Current)
	return nil
}

func (s *Session) Reset() {
	log.Println("Reset")
	s.Current = Mail{}
}

func (s *Session) Logout() error {
	log.Println("Logout")
	return nil
}

func (s *Session) EnqueueMail(mail *Mail) {
	log.Println("Enqueing mail")
	s.Queue.Mu.Lock()
	s.Queue.M <- *mail
	s.Queue.Mu.Unlock()
}

func DispatchQueue(q *MailQueue, c *Config) {
	if len(q.M) == 0 {
		return
	}
	log.Println("Dispatching mail queue of size", len(q.M))
	q.Mu.Lock()
	defer q.Mu.Unlock()
	for m := range q.M {
		if err := SendMail(&m, c); err != nil {
			log.Println("Error sending mail:", err)
			log.Println("Saving mail content to /tmp/smtpsesgw-mail")
			data, err := json.Marshal(m)
			if err != nil {
				log.Println("Error marshalling mail:", err)
				continue
			}
			tempFile, err := os.CreateTemp("/tmp", "smtpbridge-mail-*.json")
			if err != nil {
				log.Println("Error creating temp file:", err)
				continue
			}
			if _, err := tempFile.Write(data); err != nil {
				log.Println("Error writing temp file:", err)
				continue
			}
			log.Println("Wrote mail to", tempFile.Name())
		}
	}
}

func parseHeaders(data []byte) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		headers[parts[0]] = strings.Replace(parts[1], "\r", "", -1)
	}
	return headers
}

func parseBody(data []byte) string {
	// skip headers
	lines := strings.Split(string(data), "\n")
	var body []string
	for i, line := range lines {
		if line == "\r" || line == "" {
			body = lines[i+1:]
			break
		}
	}
	return strings.Join(body, "\n")
}

func GetDestList(m *Mail) []*string {
	var dest []*string
	for _, d := range m.To {
		dest = append(dest, &d)
	}
	return dest
}

func GetSubject(m *Mail) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers["Subject"]
}

func GetCharset(m *Mail) string {
	if m.Headers == nil {
		return "UTF-8"
	}
	if m.Headers["Content-Type"] == "" {
		return "UTF-8"
	}
	raw := strings.Split(m.Headers["Content-Type"], "charset=")
	if len(raw) != 2 {
		return "UTF-8"
	}
	set := strings.ReplaceAll(raw[1], "\"", "")
	return set
}
