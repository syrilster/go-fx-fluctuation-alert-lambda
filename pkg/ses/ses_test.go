package ses

import (
	"errors"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockSES struct {
	sendEmailFunc func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error)
}

func (m *mockSES) SendEmail(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
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
		input       *ses.SendEmailInput
		expected    *ses.SendEmailOutput
		expectedErr error
		wantErr     bool
	}{
		{
			description: "Success",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
					return &ses.SendEmailOutput{MessageId: dummyMsgID}, nil
				}},
			},
			input:    &ses.SendEmailInput{},
			expected: &ses.SendEmailOutput{MessageId: dummyMsgID},
			wantErr:  false,
		},
		{
			description: "Failure",
			mockClient: &Client{
				SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
					return nil, errors.New("something went wrong")
				}},
			},
			input:       &ses.SendEmailInput{},
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
