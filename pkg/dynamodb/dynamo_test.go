package dynamodb

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

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
	dummyTable := "dummyTable"
	dummyValue := "dummyValue"
	dummyItem := struct {
		MyKey string `dynamodbav:"hash" json:"hash"`
	}{
		MyKey: dummyValue,
	}

	var store = &DynamoStore{
		TableName: dummyTable,
		DB:        &MockDynamoDB{},
	}

	d := NewDBService(store)
	err := d.store.CreateItem(dummyItem)
	if err != nil {
		assert.Fail(t, "un-expected error")
	}
}

func TestDynamoDB_GetByKey(t *testing.T) {
	dummyTable := "dummyTable"
	var store = &DynamoStore{
		TableName: dummyTable,
		DB:        &MockDynamoDB{},
	}

	var item = &Item{}
	d := NewDBService(store)
	err := d.store.GetItem(dummyHash, &item)
	if err != nil {
		assert.Fail(t, "un-expected error", err)
	}
	assert.NoError(t, err)
	require.Equal(t, item.HashString, dummyHash)
}
