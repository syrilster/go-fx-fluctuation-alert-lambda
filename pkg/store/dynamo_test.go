package store

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/store/mocks"
)

type DBService struct {
	store CurrencySaver
}

func NewDBService(s CurrencySaver) *DBService {
	return &DBService{
		store: s,
	}
}

func Test_CreateItem(t *testing.T) {
	tests := []struct {
		name       string
		item       Item
		mockReturn *dynamodb.PutItemOutput
		mockError  error
		expectErr  bool
	}{
		{
			name:       "Success",
			item:       Item{HashString: "testHash", CurrencyValue: 0, Expires: 0},
			mockReturn: &dynamodb.PutItemOutput{},
			mockError:  nil,
			expectErr:  false,
		},
		{
			name:       "Failure-MissingHashString",
			item:       Item{HashString: ""},
			mockReturn: nil,
			mockError:  errors.New("missing hash string"),
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDynamo := new(mocks.DynamoDB)
			store := NewCurrencyStore("testTable", mockDynamo)

			av, err := attributevalue.MarshalMap(tt.item)
			if err != nil || av == nil {
				log.Error().Err(err).Msg("error in CreateItem to db")
				assert.FailNow(t, "error in test setup")
			}
			input := &dynamodb.PutItemInput{
				TableName: aws.String(store.table), Item: av,
			}
			// Set up mock behavior
			mockDynamo.On("PutItem", mock.Anything, input).Return(tt.mockReturn, tt.mockError)

			// Call the method
			err = store.CreateItem(tt.item)

			// Validate expectations
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_GetItem(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		mockReturn *dynamodb.GetItemOutput
		mockError  error
		expectErr  bool
		err        error
	}{
		{
			name:  "Success",
			input: "testHash",
			mockReturn: &dynamodb.GetItemOutput{
				Item: map[string]types.AttributeValue{
					"hash":           &types.AttributeValueMemberS{Value: "testHash"},
					"currency_value": &types.AttributeValueMemberN{Value: "123.45"},
					"expires_at":     &types.AttributeValueMemberN{Value: "1672531199"},
				},
			},
			mockError: nil,
			expectErr: false,
		},
		{
			name:       "Failure-WhenGetItemResponseIsEmpty",
			input:      "",
			mockReturn: &dynamodb.GetItemOutput{},
			mockError:  nil,
			expectErr:  true,
			err:        errors.New("requested item not found"),
		},
		{
			name:       "Failure-MissingHashString",
			input:      "",
			mockReturn: nil,
			mockError:  errors.New("something went wrong"),
			expectErr:  true,
			err:        errors.New("getItem error: something went wrong"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock setup
			mockDynamo := new(mocks.DynamoDB)
			store := NewCurrencyStore("testTable", mockDynamo)

			av, err := attributevalue.MarshalMap(tt.input)
			if err != nil || av == nil {
				log.Error().Err(err).Msg("error in CreateItem to db")
				assert.FailNow(t, "error in test setup")
			}

			// Set up mock behavior
			mockDynamo.On("GetItem", mock.Anything, mock.Anything).Return(tt.mockReturn, tt.mockError)

			// Initialize the service
			d := NewDBService(store)

			// Call the method
			resp, err := d.store.GetItem(tt.input)

			// Validate expectations
			if tt.expectErr {
				require.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, *resp, Item{HashString: tt.input, CurrencyValue: 123.45, Expires: 1672531199})
			}
		})
	}
}
