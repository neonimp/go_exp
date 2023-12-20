package main

import (
	"errors"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

func bodyParse(m *Mail) *ses.Body {
	if m.Headers == nil || m.Headers["Content-Type"] == "" {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(GetCharset(m)),
			},
		}
	}
	if strings.Contains(m.Headers["Content-Type"], "text/html") {
		return &ses.Body{
			Html: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(GetCharset(m)),
			},
		}
	} else {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(GetCharset(m)),
			},
		}
	}
}

func SendMail(m *Mail, c *Config) error {
	if m == nil {
		return errors.New("mail is nil")
	}

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
		ToAddresses: GetDestList(m),
	}

	// Assemble the email.
	input := &ses.SendEmailInput{
		Source:      aws.String(m.From),
		Destination: dest,
		Message: &ses.Message{
			Subject: &ses.Content{
				Data:    aws.String(GetSubject(m)),
				Charset: aws.String(GetCharset(m)),
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
