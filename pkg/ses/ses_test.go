package ses

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/stretchr/testify/require"
)

type mockSES struct {
	sendEmailFunc func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error)
}

func (m *mockSES) SendEmail(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
	return m.sendEmailFunc(ctx, input)
}

func TestNew(t *testing.T) {
	c, err := New(aws.Config{Region: "ap-south-1"})
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestSendEmail(t *testing.T) {
	var dummyMsgID = new(string)
	*dummyMsgID = "1234"
	tests := []struct {
		name        string
		mockClient  *Client
		input       *sesv2.SendEmailInput
		expected    *sesv2.SendEmailOutput
		expectedErr error
		wantErr     bool
	}{
		{
			name: "Success",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
					return &sesv2.SendEmailOutput{MessageId: dummyMsgID}, nil
				}},
			},
			input:    &sesv2.SendEmailInput{},
			expected: &sesv2.SendEmailOutput{MessageId: dummyMsgID},
			wantErr:  false,
		},
		{
			name: "Failure",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
					return nil, errors.New("something went wrong")
				}},
			},
			input:       &sesv2.SendEmailInput{},
			expectedErr: errors.New("something went wrong"),
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mockClient.SendEmail(context.Background(), tt.input)
			if tt.wantErr {
				require.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.Equal(t, got, tt.expected)
			}
		})
	}
}
