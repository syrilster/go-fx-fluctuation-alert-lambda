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

func New(t string, db *dynamodb.DynamoDB) *DynamoStore {
	return &DynamoStore{
		TableName: t,
		DB:        db,
	}
}

// Create write new entry into the DB table
func Create(db dynamodbiface.DynamoDBAPI, table string, item interface{}) error {
	av, err := dynamodbattribute.Marshal(item)
	if err != nil || av == nil {
		log.Error().Err(err).Msg("error in Create to db")
		return err
	}
	return create(db, table, av)
}

func create(db dynamodbiface.DynamoDBAPI, table string, av *dynamodb.AttributeValue) error {
	_, err := db.PutItem(&dynamodb.PutItemInput{Item: av.M, TableName: &table})
	return err
}

// GetItem gets items from a DynamoDB table based on a provided hash
func GetItem(db dynamodbiface.DynamoDBAPI, table string, hash string, item interface{}) error {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			"hash": {
				S: aws.String(hash),
			},
		},
	}
	output, err := db.GetItem(input)
	if err != nil {
		return err
	}

	if len(output.Item) == 0 {
		return fmt.Errorf("item not found")
	}

	return dynamodbattribute.UnmarshalMap(output.Item, &item)
}
