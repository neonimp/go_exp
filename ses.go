package main

import (
	"errors"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

func mailDestLister(m *Mail) []*string {
	var dest []*string
	for _, d := range m.To {
		dest = append(dest, aws.String(d))
	}
	return dest
}

func getSubject(m *Mail) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers["Subject"]
}

func getCharset(m *Mail) string {
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

func bodyParse(m *Mail) *ses.Body {
	if m.Headers == nil || m.Headers["Content-Type"] == "" {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(getCharset(m)),
			},
		}
	}
	if strings.Contains(m.Headers["Content-Type"], "text/html") {
		return &ses.Body{
			Html: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(getCharset(m)),
			},
		}
	} else {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(getCharset(m)),
			},
		}
	}
}

func SendMail(m *Mail, c *Config) error {
	sess, err := session.NewSession(&aws.Config{
		Region:                        aws.String(c.Ses.Region),
		CredentialsChainVerboseErrors: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	// Create the SES session.
	svc := ses.New(sess)
	dest := &ses.Destination{
		ToAddresses: mailDestLister(m),
	}

	// Assemble the email.
	input := &ses.SendEmailInput{
		Source:      aws.String(m.From),
		Destination: dest,
		Message: &ses.Message{
			Subject: &ses.Content{
				Data:    aws.String(getSubject(m)),
				Charset: aws.String(getCharset(m)),
			},
			Body: bodyParse(m),
		},
	}

	if c.DryMode {
		log.Println("Dry mode enabled, not sending mail")
		return errors.New("dry mode enabled")
	}

	// Attempt to send the email.
	_, err = svc.SendEmail(input)
	return err
}
