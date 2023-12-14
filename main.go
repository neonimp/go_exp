package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-smtp"
	"github.com/urfave/cli/v2"
)

type MailQueue struct {
	Mu sync.Mutex
	M  chan Mail
}

type Config struct {
	DispatchInterval int `toml:"dispatch_interval"`
	Smtp             struct {
		Net  string `toml:"net"`
		Host string `toml:"host"`
		Port int    `toml:"port"`
	} `toml:"smtp"`
	Auth struct {
		AuthUsers []string `toml:"auth_users"`
		AllowAnon bool     `toml:"allow_anon"`
	} `toml:"auth"`
	Ses struct {
		Region string `toml:"region"`
	} `toml:"ses"`
}

func main() {
	cli := cli.App{
		Name:  "AWS SES SMTP to API Gateway",
		Usage: "smtpsesgw",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "Path to config file",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			cfgPath := c.String("config")
			if cfgPath == "" {
				cfgPath = "/etc/smtpsesgw/config.toml"
			}

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			return run(cfg)
		},
	}

	if err := cli.Run(os.Args); err != nil {
		panic(err)
	}
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Run the bridge smtp server
func run(cfg *Config) error {
	queue := &MailQueue{
		Mu: sync.Mutex{},
		M:  make(chan Mail, 100),
	}

	be := &Backend{
		Cfg:   cfg,
		Queue: queue,
	}
	srv := smtp.NewServer(be)

	srv.Addr = cfg.Smtp.Host + ":" + fmt.Sprintf("%d", cfg.Smtp.Port)
	srv.Domain = cfg.Smtp.Host
	srv.MaxMessageBytes = 256 * int64(math.Pow(2, 20)) // 256 MiB
	srv.MaxRecipients = 500
	srv.AllowInsecureAuth = true

	stop := make(chan bool)
	// run the mail queue dispatcher every 5 seconds
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.DispatchInterval) * time.Second)
		for {
			select {
			case <-ticker.C:
				dispatchQueue(queue)
			case <-stop:
				log.Println("Stopping mail queue dispatcher")
				ticker.Stop()
				close(queue.M)
				close(stop)
				return
			}
		}
	}()

	log.Println("Starting server at", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

type Backend struct {
	Cfg   *Config
	Queue *MailQueue
}

type Mail struct {
	From     string
	To       []string
	MailData []byte
}

type Session struct {
	Cfg      *Config
	Queue    *MailQueue
	IsAuthed bool
	AuthUser string
	Current  Mail
	Mails    []Mail
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
		log.Println("Data:", string(b))
		s.Current.MailData = b
	}
	s.enqueMail(&s.Current)
	return nil
}

func (s *Session) Reset() {
	log.Println("Reset")
	s.Current = Mail{}
	s.Mails = []Mail{}
}

func (s *Session) Logout() error {
	log.Println("Logout")
	return nil
}

func (s *Session) enqueMail(mail *Mail) {
	log.Println("Enqueing mail")
	s.Queue.Mu.Lock()
	s.Queue.M <- *mail
	s.Queue.Mu.Unlock()
}

func dispatchQueue(q *MailQueue) {
	if len(q.M) == 0 {
		return
	}
	log.Println("Dispatching mail queue of size", len(q.M))
	q.Mu.Lock()
	defer q.Mu.Unlock()
	for m := range q.M {
		if err := sesSendMail(m.From, m.To, m.MailData); err != nil {
			log.Println("Error sending mail:", err)
			log.Println("Saving mail content to /tmp/smtpsesgw-mail")
			data, err := json.Marshal(m)
			if err != nil {
				log.Println("Error marshalling mail:", err)
				continue
			}
			os.WriteFile("/tmp/smtpsesgw-mail", data, 0644)
		}
	}
}

// TODO: Implement SES send mail
func sesSendMail(from string, to []string, data []byte) error {
	log.Println("Sending mail to SES")
	return nil
}
