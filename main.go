package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	DispatchInterval int  `toml:"dispatch_interval"`
	DryMode          bool `toml:"dry_mode"`
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

type Rangeable interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

func inRange[T Rangeable](v T, min T, max T) bool {
	return v >= min && v <= max
}

func main() {
	cli := cli.App{
		Name:  "SMTP to API Bridge",
		Usage: "smtpsesgw",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Usage:    "Path to config file",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "port",
				Aliases:  []string{"p"},
				Usage:    "SMTP port to listen on",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "host",
				Aliases:  []string{"H"},
				Usage:    "SMTP host to listen on",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			cfgPath := c.String("config")
			if cfgPath == "" {
				cfgPath = "/etc/smtpbridge/config.toml"
			}

			cfg, err := loadConfig(cfgPath)
			if v := c.Int("port"); v != 0 {
				if !inRange(v, 1, 65535) {
					return fmt.Errorf("port must be between 1 and 65535")
				}
				cfg.Smtp.Port = v
			}
			if v := c.String("host"); v != "" {
				cfg.Smtp.Host = v
			}

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
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM, syscall.SIGINT)

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

	// run the mail queue dispatcher every 5 seconds
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.DispatchInterval) * time.Second)
		for {
			select {
			case <-ticker.C:
				go DispatchQueue(queue, cfg)
			case <-shutdownChan:
				log.Println("Stopping mail queue dispatcher")
				ticker.Stop()
				close(queue.M)
				close(shutdownChan)
				shutdownCtx := context.Background()
				shutdownCtx, cancelFunc := context.WithDeadline(shutdownCtx, time.Now().Add(5*time.Second))
				defer cancelFunc()
				srv.Shutdown(shutdownCtx)
				return
			}
		}
	}()

	log.Println("Starting server at", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return err
	}
	log.Println("Shutdown complete, exiting")
	return nil
}
