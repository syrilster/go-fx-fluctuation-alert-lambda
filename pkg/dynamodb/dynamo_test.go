package dynamodb

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	dummyHash    = "12345"
	dummyCurrVal = "59"
)

type Item struct {
	HashString    string  `json:"hash"`
	CurrencyValue float32 `json:"currency_value"`
	Expires       int64   `json:"expires_at"`
}

func TestDynamoDB_Create(t *testing.T) {
	dummyTable := "dummyTable"
	dummyValue := "dummyValue"
	dummyItem := struct {
		MyKey string `dynamodbav:"myKey" json:"my_key"`
	}{
		MyKey: dummyValue,
	}
	var mockDynamoDB = MockDynamoDB{
		PutItemFn: func(input *dynamodb.PutItemInput) (output *dynamodb.PutItemOutput, e error) {
			if *input.TableName != dummyTable {
				assert.Fail(t, "table name mismatch")
			}

			if *input.Item["myKey"].S != dummyValue {
				assert.Fail(t, "key name mismatch")
			}
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	err := Create(&mockDynamoDB, dummyTable, dummyItem)
	if err != nil {
		assert.Fail(t, "un-expected error")
	}
}

func TestDynamoDB_GetByKey(t *testing.T) {
	dummyTable := "fx_rate"
	var mockDynamoDB = MockDynamoDB{
		GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
			assert.Equal(t, dummyTable, *input.TableName)
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
	}

	var item = &Item{}
	err := GetItem(&mockDynamoDB, dummyTable, "this-index", &item)
	if err != nil {
		assert.Fail(t, "un-expected error", err)
	}
	assert.NoError(t, err)
	require.Equal(t, item.HashString, dummyHash)
}
