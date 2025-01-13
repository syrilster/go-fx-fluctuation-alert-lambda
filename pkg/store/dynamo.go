package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CurrencyStore struct {
	table    string
	dynamoDb dynamoDb
}

//go:generate mockery --name dynamoDb --structname DynamoDB
type dynamoDb interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

type CurrencySaver interface {
	CreateItem(item Item) error
	GetItem(hash string) (*Item, error)
}

func NewCurrencyStore(t string, db dynamoDb) *CurrencyStore {
	return &CurrencyStore{
		table:    t,
		dynamoDb: db,
	}
}

// CreateItem write new entry into the DB table
func (d *CurrencyStore) CreateItem(item Item) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil || av == nil {
		return err
	}

	_, err = d.dynamoDb.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(d.table), Item: av,
	})
	if err != nil {
		return err
	}
	return nil
}

// GetItem gets items from a DynamoDB table based on a provided hash
func (d *CurrencyStore) GetItem(hash string) (*Item, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key: map[string]types.AttributeValue{
			"hash": &types.AttributeValueMemberS{Value: hash},
		},
	}

	response, err := d.dynamoDb.GetItem(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("getItem error: %w", err)
	}

	// Check if the item exists
	if len(response.Item) == 0 {
		return nil, fmt.Errorf("requested item not found")
	}

	// Unmarshal the item into the provided struct
	item := &Item{}
	err = attributevalue.UnmarshalMap(response.Item, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return item, nil
}
