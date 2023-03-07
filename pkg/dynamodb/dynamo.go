package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/rs/zerolog/log"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type DynamoStore struct {
	TableName string
	DB        dynamodbiface.DynamoDBAPI
}

type DBStore interface {
	CreateItem(item interface{}) error
	GetItem(hash string, item interface{}) error
}

func NewStore(t string, db *dynamodb.DynamoDB) *DynamoStore {
	return &DynamoStore{
		TableName: t,
		DB:        db,
	}
}

// CreateItem write new entry into the DB table
func (d *DynamoStore) CreateItem(item interface{}) error {
	av, err := dynamodbattribute.Marshal(item)
	if err != nil || av == nil {
		log.Error().Err(err).Msg("error in CreateItem to db")
		return err
	}
	return create(d.DB, d.TableName, av)
}

func create(db dynamodbiface.DynamoDBAPI, table string, av *dynamodb.AttributeValue) error {
	_, err := db.PutItem(&dynamodb.PutItemInput{Item: av.M, TableName: &table})
	return err
}

// GetItem gets items from a DynamoDB table based on a provided hash
func (d *DynamoStore) GetItem(hash string, item interface{}) error {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"hash": {
				S: aws.String(hash),
			},
		},
	}
	output, err := d.DB.GetItem(input)
	if err != nil {
		return err
	}

	if len(output.Item) == 0 {
		return fmt.Errorf("item not found")
	}

	return dynamodbattribute.UnmarshalMap(output.Item, &item)
}
