package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/emersion/go-smtp"
	"github.com/urfave/cli/v2"
)

type Config struct {
	Smtp struct {
		Net  string `toml:"net"`
		Host string `toml:"host"`
		Port int    `toml:"port"`
	} `toml:"smtp"`
	Auth struct {
		AuthUsers []string `toml:"auth_users"`
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
	be := &Backend{
		Cfg: cfg,
	}
	srv := smtp.NewServer(be)

	srv.Addr = cfg.Smtp.Host + ":" + fmt.Sprintf("%d", cfg.Smtp.Port)
	srv.Domain = cfg.Smtp.Host
	srv.MaxMessageBytes = 256 * int64(math.Pow(2, 20)) // 256 MiB
	srv.MaxRecipients = 500
	srv.AllowInsecureAuth = true

	log.Println("Starting server at", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

type Backend struct {
	Cfg *Config
}

type Mail struct {
	From     string
	To       []string
	MailData []byte
}

type Session struct {
	Cfg      *Config
	IsAuthed bool
	AuthUser string
	Current  Mail
	Mails    []Mail
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		Cfg:      b.Cfg,
		IsAuthed: false,
		AuthUser: "",
		Current:  Mail{},
		Mails:    []Mail{},
	}, nil
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
	return nil
}

func (s *Session) Reset() {
	log.Println("Queuing mail")
	s.Mails = append(s.Mails, s.Current)
	s.Current = Mail{}
}

func (s *Session) Logout() error {
	log.Println("Logout user from SMTP", s.AuthUser)
	if !s.IsAuthed {
		log.Println("Not authenticated")
		return nil
	}
	if len(s.Mails) == 0 {
		log.Println("No mails to send")
		return nil
	}
	log.Println("Sending", len(s.Mails), "mails")
	for _, m := range s.Mails {
		log.Println("Sending mail from", m.From, "to", m.To)
		if err := sesSendMail(m.From, m.To, m.MailData); err != nil {
			return err
		}
	}
	return nil
}

// TODO: Implement SES send mail
func sesSendMail(from string, to []string, data []byte) error {
	return nil
}
