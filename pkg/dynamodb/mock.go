package dynamodb

import (
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// MockDynamoDB implements the aws dynamodb interface and allows for specifying mocking behavior
type MockDynamoDB struct {
	BatchGetItemFn   func(*dynamodb.BatchGetItemInput) (*dynamodb.BatchGetItemOutput, error)
	BatchWriteItemFn func(*dynamodb.BatchWriteItemInput) (*dynamodb.BatchWriteItemOutput, error)
	DeleteItemFn     func(*dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error)
	GetItemFn        func(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	PutItemFn        func(*dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
	QueryFn          func(*dynamodb.QueryInput) (*dynamodb.QueryOutput, error)
	ScanFn           func(*dynamodb.ScanInput) (*dynamodb.ScanOutput, error)
	UpdateItemFn     func(*dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error)

	dynamodbiface.DynamoDBAPI
}

// BatchGetItem operation returns the attributes of one or more items from one or more tables.
func (db *MockDynamoDB) BatchGetItem(item *dynamodb.BatchGetItemInput) (*dynamodb.BatchGetItemOutput, error) {
	return db.BatchGetItemFn(item)
}

// BatchWriteItem operation puts or deletes multiple items in one or more tables.
func (db *MockDynamoDB) BatchWriteItem(item *dynamodb.BatchWriteItemInput) (*dynamodb.BatchWriteItemOutput, error) {
	return db.BatchWriteItemFn(item)
}

// DeleteItem deletes  a single item in a table by primary key.
func (db *MockDynamoDB) DeleteItem(item *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	return db.DeleteItemFn(item)
}

// GetItem operation returns a set of attributes for the item with the given primary key.
func (db *MockDynamoDB) GetItem(item *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	return db.GetItemFn(item)
}

// PutItem creates a new item, or replaces an old item with a new item.
func (db *MockDynamoDB) PutItem(item *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	return db.PutItemFn(item)
}

// Query finds items based on primary key values.
func (db *MockDynamoDB) Query(input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	return db.QueryFn(input)
}

// Scan returns one or more items and item attributes by accessing every item in a table or a secondary index
func (db *MockDynamoDB) Scan(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	return db.ScanFn(input)
}

// UpdateItem edits an existing item's attributes, or adds a new item to the table if it does not already exist.
func (db *MockDynamoDB) UpdateItem(input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	return db.UpdateItemFn(input)
}
