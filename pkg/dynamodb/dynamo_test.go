package dynamodb

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDynamo struct {
	store          DBStore
	createItemCall func(item interface{}) error
	getItemCall    func(hash string, item interface{}) error
}

func (m *mockDynamo) CreateItem(item interface{}) error {
	return m.createItemCall(item)
}

func (m *mockDynamo) GetItem(hash string, item interface{}) error {
	return m.getItemCall(hash, item)
}

type DBService struct {
	store DBStore
}

type Item struct {
	HashString    string  `json:"hash"`
	CurrencyValue float32 `json:"currency_value"`
	Expires       int64   `json:"expires_at"`
}

func NewDBService(s DBStore) *DBService {
	return &DBService{
		store: s,
	}
}

func TestDynamoDB_Create(t *testing.T) {
	dummyValue := "dummyValue"
	type dummyItem struct {
		MyKey string `dynamodbav:"hash" json:"hash"`
	}

	var store = &mockDynamo{
		createItemCall: func(item interface{}) error {
			i, ok := item.(dummyItem)
			if !ok {
				assert.Fail(t, "something went wrong")
				return errors.New("type conversion failed")
			}
			assert.Equal(t, i.MyKey, dummyValue)
			return nil
		},
	}

	d := NewDBService(store)
	err := d.store.CreateItem(dummyItem{MyKey: dummyValue})
	if err != nil {
		assert.Fail(t, "un-expected error")
	}
}

func TestDynamoDB_GetByKey(t *testing.T) {

	tests := []struct {
		name      string
		store     *DynamoStore
		expectErr bool
	}{
		{
			name: "Success",
			store: &DynamoStore{
				TableName: dummyTable,
				DB: &MockDynamoDB{
					GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
						if *input.TableName != dummyTable {
							assert.Fail(t, "table name mismatch")
						}

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
			},
		},
		{
			name: "FailedToGetItem",
			store: &DynamoStore{
				TableName: dummyTable,
				DB: &MockDynamoDB{
					GetItemFn: func(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
						if *input.TableName != dummyTable {
							assert.Fail(t, "table name mismatch")
						}

						return nil, errors.New("dynamo unknown error")
					},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			item := &Item{}
			d := NewDBService(tt.store)
			err := d.store.GetItem(dummyHash, &item)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.Equal(t, item.HashString, dummyHash)
			}
		})
	}
}
