package ses

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/rs/zerolog/log"
)

type SES interface {
	SendEmail(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error)
}

type Client struct {
	SES
}

type ClientAdapter struct {
	*sesv2.Client
}

func adaptClient(s *sesv2.Client) *ClientAdapter {
	return &ClientAdapter{s}
}

func New(options sesv2.Options) (*Client, error) {
	nc := sesv2.New(options)
	c := adaptClient(nc)
	return &Client{c}, nil
}

func (c *ClientAdapter) SendEmail(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
	output, err := c.Client.SendEmail(ctx, input)
	if err != nil {
		log.Error().Err(err).Msg("error when sending email")
		return nil, err
	}
	return output, nil
}
