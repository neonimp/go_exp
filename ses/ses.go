package ses

import (
	"errors"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/neonimp/smtpbridge/backend"
	"github.com/neonimp/smtpbridge/config"
)

type SesConfig struct {
	Region  string
	Profile string
}

func bodyParse(m *backend.Mail) *ses.Body {
	if m.Headers == nil || m.Headers["Content-Type"] == "" {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(m.GetCharset()),
			},
		}
	}
	if strings.Contains(m.Headers["Content-Type"], "text/html") {
		return &ses.Body{
			Html: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(m.GetCharset()),
			},
		}
	} else {
		return &ses.Body{
			Text: &ses.Content{
				Data:    aws.String(m.Body),
				Charset: aws.String(m.GetCharset()),
			},
		}
	}
}

func SendMail(m *backend.Mail, c *config.Config) error {
	reg, ok := c.GetProviderStringSetting("region")
	if !ok {
		return errors.New("region not set")
	}
	if m == nil {
		return errors.New("mail is nil")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:                        aws.String(reg),
		CredentialsChainVerboseErrors: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	// Create the SES session.
	svc := ses.New(sess)
	dest := &ses.Destination{
		ToAddresses: m.GetDestList(),
	}

	// Assemble the email.
	input := &ses.SendEmailInput{
		Source:      aws.String(m.From),
		Destination: dest,
		Message: &ses.Message{
			Subject: &ses.Content{
				Data:    aws.String(m.GetSubject()),
				Charset: aws.String(m.GetCharset()),
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
