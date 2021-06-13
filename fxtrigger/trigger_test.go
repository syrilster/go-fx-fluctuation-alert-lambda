package fxtrigger

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/exchange"
	dynamo "github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/dynamodb"
	pses "github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/ses"
	"io/ioutil"
	"os"
	"testing"
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

type mockExchange struct {
	GetExchangeRateFunc func(ctx context.Context, request exchange.Request) (float32, error)
}

func (m *mockExchange) GetExchangeRate(ctx context.Context, request exchange.Request) (float32, error) {
	return m.GetExchangeRateFunc(ctx, request)
}

type mockSES struct {
	sendEmailFunc func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error)
}

func (m *mockSES) SendEmail(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
	return m.sendEmailFunc(input)
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
	}{
		{name: "success",
			configPath: testYAMLFile.Name()},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			//require.NoError(t, os.Setenv(configPathKey, tt.configPath))
			//err := Handler(context.Background(), CustomEvent{})
			//
			//require.NoError(t, err)
		})
	}
}

func TestProcess(t *testing.T) {
	var dummyStore = &dynamo.DynamoStore{
		TableName: dummyTable,
		DB: &dynamo.MockDynamoDB{
			GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
				return &dynamodb.GetItemOutput{
					Item: map[string]*dynamodb.AttributeValue{
						"hash": {
							S: aws.String(dummyHash),
						},
						"currency_value": {
							N: aws.String(dummyCurrVal),
						},
					},
				}, nil
			},
		},
	}

	var dummyFailStore = &dynamo.DynamoStore{
		TableName: dummyTable,
		DB: &dynamo.MockDynamoDB{
			GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
				return nil, errors.New("dynamo unknown error")
			},
			PutItemFn: func(input *dynamodb.PutItemInput) (output *dynamodb.PutItemOutput, e error) {
				if *input.TableName != dummyTable {
					assert.Fail(t, "table name mismatch")
				}

				if *input.Item["hash"].S == "" {
					assert.Fail(t, "key name mismatch")
				}
				return nil, errors.New("dynamo unknown error")
			},
		},
	}

	tests := []struct {
		name      string
		sesClient *pses.Client
		store     *dynamo.DynamoStore
		eClient   exchange.ClientInterface
		request   exchange.Request
		expectErr bool
	}{
		{
			name:  "Success - When Hash entry already in DB",
			store: dummyStore,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
		},
		{
			name:  "Success - When exchange rate meets lower bound",
			store: dummyStore,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 50, nil
			}},
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
				return &ses.SendEmailOutput{}, nil
			}}},
		},
		{
			name:  "Success - When threshold not met",
			store: dummyStore,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 56, nil
			}},
		},
		{
			name: "Success - When Hash not in DB",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
				return &ses.SendEmailOutput{}, nil
			}}},
			store: &dynamo.DynamoStore{
				TableName: dummyTable,
				DB: &dynamo.MockDynamoDB{
					GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
						return nil, errors.New("dynamo unknown error")
					},
					PutItemFn: func(input *dynamodb.PutItemInput) (output *dynamodb.PutItemOutput, e error) {
						if *input.TableName != dummyTable {
							assert.Fail(t, "table name mismatch")
						}

						if *input.Item["hash"].S == "" {
							assert.Fail(t, "key name mismatch")
						}
						return &dynamodb.PutItemOutput{}, nil
					},
				},
			},
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
		},
		{
			name: "FailureWhenCreateOpDB",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
				return &ses.SendEmailOutput{}, nil
			}}},
			store: dummyFailStore,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
			expectErr: true,
		},
		{
			name: "FailureWhenSESError",
			sesClient: &pses.Client{SES: &mockSES{sendEmailFunc: func(input *ses.SendEmailInput) (*ses.SendEmailOutput, error) {
				return nil, errors.New("something went wrong")
			}}},
			store: dummyFailStore,
			eClient: &mockExchange{GetExchangeRateFunc: func(ctx context.Context, request exchange.Request) (float32, error) {
				return 59, nil
			}},
			expectErr: true,
		},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			err := process(context.Background(), dummyConfig, tt.store, tt.sesClient, tt.eClient, tt.request)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func createTempFile(fileName string, content []byte) (f *os.File, err error) {
	file, _ := ioutil.TempFile(os.TempDir(), fileName)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	_, err = file.Write(content)
	if err != nil {
		return nil, err
	}

	return file, nil
}
