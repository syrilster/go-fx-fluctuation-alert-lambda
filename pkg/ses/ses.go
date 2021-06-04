package ses

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/rs/zerolog/log"
)

type SES interface {
	SendEmail(input *ses.SendEmailInput) (*ses.SendEmailOutput, error)
}

type ClientAdapter struct {
	*ses.SES
}

func New(awsRegion string) (*ClientAdapter, error) {
	s, err := session.NewSession(aws.NewConfig().WithRegion(awsRegion))
	if err != nil {
		log.Error().Err(err).Msg("error getting a SES session")
		return nil, err
	}
	c := ses.New(s)
	return &ClientAdapter{c}, nil
}

func (c *ClientAdapter) SendEmail(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
	output, err := c.SES.SendEmail(input)
	if err != nil {
		log.Error().Err(err).Msg("error when sending email")
		return nil, err
	}
	return output, nil
}
