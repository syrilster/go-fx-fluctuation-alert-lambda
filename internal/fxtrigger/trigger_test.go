package fxtrigger

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/exchange"
	pses "github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/ses"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/store"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/store/mocks"
)

const (
	dummyHash    = "12345"
	dummyCurrVal = "59"
	dummyTable   = "fx_rate"
)

var testYaml = `
toEmail: test@gmail.com
`
var testYAMLFile, _ = createTempFile("config.yaml", []byte(testYaml))

var dummyConfig = &Config{
	ToEmail:          "test@gmail.com",
	FromEmail:        "test@gmail.com",
	AWSRegion:        "dummy-region",
	ThresholdPercent: 5,
	LowerBound:       55,
	UpperBound:       58,
}

type MockCurrencySaver struct {
	CreateItemFn func(item store.Item) error
	GetItemFn    func(hash string) (*store.Item, error)
}

func (m *MockCurrencySaver) CreateItem(item store.Item) error {
	if m.CreateItemFn != nil {
		return m.CreateItemFn(item)
	}
	return nil
}

// GetItem mocks the GetItem method.
func (m *MockCurrencySaver) GetItem(hash string) (*store.Item, error) {
	if m.GetItemFn != nil {
		return m.GetItemFn(hash)
	}
	return nil, nil
}

type mockExchange struct {
	GetExchangeRateFunc func(ctx context.Context, request exchange.Request) (float32, error)
}

func (m *mockExchange) GetExchangeRate(ctx context.Context, request exchange.Request) (float32, error) {
	return m.GetExchangeRateFunc(ctx, request)
}

type mockSES struct {
	sendEmailFunc func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error)
}

func (m *mockSES) SendEmail(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
	return m.sendEmailFunc(ctx, input)
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
	}{
		{
			name:       "success",
			configPath: testYAMLFile.Name(),
		},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv(configPathKey, tt.configPath))
			require.NoError(t, os.Setenv(lowerBound, "50"))
			require.NoError(t, os.Setenv(upperBound, "55"))
			err := Handler(context.Background(), CustomEvent{})

			require.NotNil(t, err)
		})
	}
}

func TestHandlerFailure(t *testing.T) {
	t.Run("failWhenNoLowerBoundInEnv", func(t *testing.T) {
		require.NoError(t, os.Setenv(configPathKey, testYAMLFile.Name()))
		err := Handler(context.Background(), CustomEvent{})
		require.Error(t, err)
	})

	t.Run("failWhenNoUpperBoundInEnv", func(t *testing.T) {
		require.NoError(t, os.Setenv(configPathKey, testYAMLFile.Name()))
		require.NoError(t, os.Setenv(lowerBound, "50"))
		err := Handler(context.Background(), CustomEvent{})
		require.Error(t, err)
	})
}

func TestProcess(t *testing.T) {
	//store := &MockCurrencySaver{
	//	GetItemFn: func(hash string) (*dynamo.Item, error) {
	//		if hash == "" {
	//			return nil, errors.New("item not found")
	//		}
	//		item := dynamo.Item{HashString: hash}
	//		return &item, nil
	//	},
	//}

	tests := []struct {
		name       string
		sesClient  *pses.Client
		eClient    exchange.ClientInterface
		request    exchange.Request
		mockReturn *dynamodb.GetItemOutput
		mockError  error
		putItemErr bool
		expectErr  bool
		err        error
	}{
		{
			name: "Success - When Hash entry already in DB",
			mockReturn: &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"hash":           &types.AttributeValueMemberS{Value: "testHash"},
					"currency_value": &types.AttributeValueMemberN{Value: "123.45"},
					"expires_at":     &types.AttributeValueMemberN{Value: "1672531199"},
				},
			},
			mockError: nil,
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
				return &sesv2.SendEmailOutput{}, nil
			}}},
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
		},
		{
			name: "Success - When exchange rate meets lower bound",
			mockReturn: &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"hash":           &types.AttributeValueMemberS{Value: "testHash"},
					"currency_value": &types.AttributeValueMemberN{Value: "123.45"},
					"expires_at":     &types.AttributeValueMemberN{Value: "1672531199"},
				},
			},
			mockError: nil,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 50, nil
			}},
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
				return &sesv2.SendEmailOutput{}, nil
			}}},
		},
		{
			name:       "Success - When threshold not met",
			mockReturn: &dynamodb.GetItemOutput{},
			mockError:  nil,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 56, nil
			}},
		},
		{
			name: "Success - When Hash not in DB",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
				return &sesv2.SendEmailOutput{}, nil
			}}},
			mockReturn: &dynamodb.GetItemOutput{},
			mockError:  errors.New("something went wrong"),
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
		},
		{
			name: "FailureWhenExchangeError",
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 0, errors.New("something went wrong")
			}},
			expectErr: true,
			err:       errors.New("failed to get the exchange rate: something went wrong"),
		},
		{
			name: "FailureWhenCreateOpDB",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
				return &sesv2.SendEmailOutput{}, nil
			}}},
			mockReturn: &dynamodb.GetItemOutput{},
			mockError:  errors.New("something went wrong"),
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
			putItemErr: true,
			expectErr:  true,
			err:        errors.New("failed to check threshold: something went wrong"),
		},
		{
			name: "FailureWhenSESError",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(ctx context.Context, input *sesv2.SendEmailInput) (*sesv2.SendEmailOutput, error) {
				return nil, errors.New("something went wrong")
			}}},
			mockReturn: &dynamodb.GetItemOutput{},
			mockError:  errors.New("something went wrong"),
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
			expectErr: true,
			err:       errors.New("failed to send email: something went wrong"),
		},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			var putItemErr error
			mockDynamo := new(mocks.DynamoDB)
			s := store.NewCurrencyStore("t", mockDynamo)
			// Set up mock behavior
			mockDynamo.On("GetItem", mock.Anything, mock.Anything).Return(tt.mockReturn, tt.mockError)
			if tt.putItemErr {
				putItemErr = errors.New("something went wrong")
			}
			mockDynamo.On("PutItem", mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, putItemErr)

			err := process(context.Background(), dummyConfig, s, tt.sesClient, tt.eClient, tt.request)
			if tt.expectErr {
				require.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func createTempFile(fileName string, content []byte) (f *os.File, err error) {
	file, err := os.CreateTemp(os.TempDir(), fileName)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	// Write content to the temporary file
	_, err = file.Write(content)
	if err != nil {
		return nil, err
	}

	return file, nil
}
