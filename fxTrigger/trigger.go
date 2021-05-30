package fxTrigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/ses"
)

type CustomEvent struct {
	Name string `json:"name"`
}

const (
	tableName     = "fx_rate"
	awsRegion     = "ap-south-1"
	ec2Host       = "http://ec2-3-7-183-203.ap-south-1.compute.amazonaws.com/"
	configPathKey = "CONFIG_PATH"
)

var emailText = "HIGH"

type ExchangeResponse struct {
	From   string `json:"from"`
	To     string `json:"to"`
	FXRate string `json:"conversion_multiple"`
}

type Item struct {
	HashString    string  `json:"hash"`
	CurrencyValue float64 `json:"currency_value"`
	Expires       int64   `json:"expires_at"`
}

var toEmail string
var fromCurrency string
var toCurrency string
var emailClient *ses.SES
var thresholdPercentage float64
var currLowerBound float64
var currUpperBound float64
var sendEmail bool
var dbAmount float64
var fxAmount float64

//Handler func for lambda
func Handler(ctx context.Context, request CustomEvent) error {
	contextLogger := log.Ctx(ctx)
	contextLogger.Info().Msgf("Inside the lambda handler at date: %s", getLocalTime())
	contextLogger.Info().Msgf("Event Trigger: %s", request.Name)

	cfgPath := os.Getenv(configPathKey)

	var c Config
	cfg := c.getConfig(cfgPath)
	fmt.Println(cfg.ToEmail)
	s, err := session.NewSession(aws.NewConfig().WithRegion(awsRegion))
	if err != nil {
		log.Error().Err(err).Msg("error getting a SES session")
	}
	emailClient = ses.New(s)

	exchangeResponse, err := getExchangeRate(ctx)
	if err != nil {
		contextLogger.Error().Err(err).Msg("Error when getting the exchange rate")
		return errors.New("error when getting the exchange rate")
	}

	fxAmount, err = strconv.ParseFloat(exchangeResponse.FXRate, 64)
	if err != nil {
		return errors.New("error during un marshalling the FX rate")
	}

	if fxAmount >= currUpperBound || fxAmount <= currLowerBound {
		contextLogger.Info().Msg("FX threshold satisfied")
		contextLogger.Info().Msgf("Current FX rate %v", fxAmount)
		if fxAmount <= currLowerBound {
			emailText = "LOW"
		}

		hashString := fmt.Sprint(hash())
		contextLogger.Info().Msgf("computed hash is %v", hashString)
		dbItem, err := getItem(hashString)
		if err != nil {
			contextLogger.Error().Err(err).Msg("key not found in DynamoDB")
			contextLogger.Info().Msgf("Creating an item in Dynamo with computed hash")
			createItem(hashString, fxAmount)
			sendEmail = true
			dbAmount = fxAmount
		}

		if dbItem != nil {
			contextLogger.Info().Msgf("Found item in DB by hash value")
			dbAmount = dbItem.CurrencyValue
		}

		if thresholdExceedsPercentVal(fxAmount, dbAmount) {
			sendEmail = true
		}

	} else {
		contextLogger.Info().Msgf("FX Alert threshold not met")
		contextLogger.Info().Msgf("Current FX rate %v", fxAmount)
	}

	if sendEmail {
		contextLogger.Info().Msgf("Attempting to send email notification")
		err := sesSendEmail(ctx, fxAmount)
		if err != nil {
			return errors.New("error when sending email")
		}
	}

	return nil
}

func thresholdExceedsPercentVal(currentVal, existingVal float64) bool {
	if currentVal == existingVal {
		return false
	}

	fmt.Println("Inside threshold func to check if threshold is greater than set percentage i.e ", thresholdPercentage)
	diff := math.Abs(currentVal - existingVal)
	delta := (diff / existingVal) * 100
	fmt.Println("percent diff with prev value is: ", delta)
	return delta > thresholdPercentage
}

func createItem(hash string, amount float64) {
	fmt.Println("Inside the dynamo create item func")
	s, err := session.NewSession(aws.NewConfig().WithRegion(awsRegion))
	if err != nil {
		log.Error().Err(err).Msg("error getting a DB session")
	}
	svc := dynamodb.New(s)
	expires := getExpiryTime()
	rec := Item{
		hash,
		amount,
		expires,
	}

	av, err := dynamodbattribute.MarshalMap(rec)
	if err != nil {
		panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
	}

	params := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = svc.PutItem(params)
	if err != nil {
		fmt.Println(err.Error())
		panic(fmt.Sprintf("got error calling put item, %v", err))
	}

}

func getItem(hash string) (*Item, error) {
	fmt.Println("Inside the dynamo get item func")
	item := &Item{}
	s, err := session.NewSession(aws.NewConfig().WithRegion(awsRegion))
	if err != nil {
		log.Error().Err(err).Msg("error getting a DB session")
	}
	svc := dynamodb.New(s)

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"hash": {
				S: aws.String(hash),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := svc.GetItem(input)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	if len(result.Item) == 0 {
		return nil, fmt.Errorf("item not found")
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return item, nil
}

func hash() uint32 {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic(fmt.Sprintf("Failed to load local time for India, %v", err))
	}
	t := time.Now().In(loc)
	localTime := t.Format("Mon Jan 2")
	//Compute the hash based on date
	h := fnv.New32a()
	_, _ = h.Write([]byte(localTime))
	return h.Sum32()
}

func sesSendEmail(ctx context.Context, amount float64) error {
	contextLogger := log.Ctx(ctx)
	emailParams := &ses.SendEmailInput{
		Message: &ses.Message{
			Subject: &ses.Content{
				Data: aws.String(fromCurrency + " to " + toCurrency + " Alert"),
			},
			Body: &ses.Body{
				Text: &ses.Content{
					Data: aws.String(fromCurrency + " to " + toCurrency + " value is " + emailText + ". Current value is " + fmt.Sprintf("%f", amount)),
				},
			},
		},
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(toEmail)},
		},
		Source: aws.String(toEmail),
	}

	_, err := emailClient.SendEmail(emailParams)
	if err != nil {
		contextLogger.Error().Err(err).Msg("error when sending email")
		return err
	}
	return nil
}

func getExchangeRate(ctx context.Context) (*ExchangeResponse, error) {
	var httpClient = &http.Client{}
	contextLogger := log.Ctx(ctx)

	httpRequest, err := http.NewRequest(http.MethodGet, buildCurrencyExchangeEndpoint(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		contextLogger.Error().Err(err).Msgf("there was an error calling the currency exchange API. %v", err)
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			fmt.Println("Error when closing:", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		contextLogger.Info().Msgf("status returned from currency exchange service %s", resp.Status)
		return nil, fmt.Errorf("currency exchange service returned status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		contextLogger.Error().Err(err).Msgf("error reading currency exchange service data resp body (%s)", err)
		return nil, err
	}

	response := &ExchangeResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		contextLogger.Error().Err(err).Msgf("there was an error un marshalling the currency exchange API resp. %v", err)
		return nil, err
	}

	return response, nil
}

func buildCurrencyExchangeEndpoint() string {
	return ec2Host + "v1/currency-exchange/from/" + fromCurrency + "/to/" + toCurrency
}

func getLocalTime() string {
	loc, err := time.LoadLocation("Australia/Melbourne")
	if err != nil {
		panic(fmt.Sprintf("Failed to load local time for Melbourne, %v", err))
	}
	t := time.Now().In(loc)
	localTime := t.Format("Mon Jan 2 15:04:05")
	return localTime
}

func getExpiryTime() int64 {
	fmt.Print("Calculating expiry time ")
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic(fmt.Sprintf("Failed to load local time for India, %v", err))
	}
	t := time.Now().In(loc)
	return t.Add(time.Duration(14) * time.Hour).Unix()
}
