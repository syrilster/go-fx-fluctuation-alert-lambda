package ses

import (
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockSES struct {
	sendEmailFunc func(input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error)
}

func (m *mockSES) SendEmail(input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
	return m.sendEmailFunc(input)
}

func TestNew(t *testing.T) {
	c, err := New("")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestSendEmail(t *testing.T) {
	var dummyMsgID = new(string)
	*dummyMsgID = "1234"
	tests := []struct {
		description string
		mockClient  *Client
		input       *sesv2.SendEmailInput
		expected    *sesv2.SendEmailOutput
		expectedErr error
		wantErr     bool
	}{
		{
			description: "Success",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
					return &sesv2.SendEmailOutput{MessageId: dummyMsgID}, nil
				}},
			},
			input:    &sesv2.SendEmailInput{},
			expected: &sesv2.SendEmailOutput{MessageId: dummyMsgID},
			wantErr:  false,
		},
		{
			description: "Failure",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
					return nil, errors.New("something went wrong")
				}},
			},
			input:       &sesv2.SendEmailInput{},
			expectedErr: errors.New("something went wrong"),
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		got, err := tt.mockClient.SendEmail(tt.input)
		if tt.wantErr {
			require.EqualError(t, err, tt.expectedErr.Error())
		} else {
			require.Equal(t, got, tt.expected)
		}
	}
}
