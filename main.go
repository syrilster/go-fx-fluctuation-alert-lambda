package main

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
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/ses"
)

func main() {
	lambda.Start(Handler)
}

type ExchangeResponse struct {
	Base  string                 `json:"base"`
	Rates map[string]interface{} `json:"rates"`
}

type CustomEvent struct {
	Name string `json:"name"`
}

type Response struct {
	amount float64
}

type Item struct {
	HashString    string  `json:"hash"`
	CurrencyValue float64 `json:"currency_value"`
	Expires       int64   `json:"expires_at"`
}

var toEmail string
var appID string
var fromCurrency string
var toCurrency string
var emailClient *ses.SES
var thresholdPercentage float64
var currLowerBound float64
var currUpperBound float64

var tableName string = "fx_rate"
var awsRegion string = "ap-south-1"
var emailText string = "HIGH"

func init() {
	var err error
	toEmail = os.Getenv("TO_EMAIL")
	appID = os.Getenv("APP_ID")
	fromCurrency = os.Getenv("FROM_CURRENCY")
	toCurrency = os.Getenv("TO_CURRENCY")
	if thresholdPercentage, err = strconv.ParseFloat(os.Getenv("THRESHOLD_PERCENT"), 64); err != nil {
		fmt.Println("error while loading env var THRESHOLD_PERCENT ", err)
	}

	if currLowerBound, err = strconv.ParseFloat(os.Getenv("LOWER_BOUND"), 64); err != nil {
		fmt.Println("error while loading env var LOWER_BOUND ", err)
	}

	if currUpperBound, err = strconv.ParseFloat(os.Getenv("UPPER_BOUND"), 64); err != nil {
		fmt.Println("error while loading env var UPPER_BOUND ", err)
	}

	emailClient = ses.New(session.New(), aws.NewConfig().WithRegion(awsRegion))
}

//Handler func for lambda
func Handler(ctx context.Context, request CustomEvent) error {
	contextLogger := log.WithContext(ctx)
	contextLogger.Infof("Inside the lambda handler func at date: ", getLocalTime())

	var sendEmail bool
	var amount float64

	exchangeResponse, err := getExchangeRate(ctx)
	if err != nil {
		contextLogger.WithError(err).Error("error when getting the exchange rate")
		return errors.New("error when getting the exchange rate")
	}

	resp := unMarshallExchangeRate(exchangeResponse)

	if resp.amount >= currUpperBound || resp.amount <= currLowerBound {
		contextLogger.Infof("FX Alert threshold satisfied")
		if resp.amount <= currLowerBound {
			emailText = "LOW"
		}

		contextLogger.Infof("hash is %v", hash())
		hashString := fmt.Sprint(hash())
		dbItem, err := getItem(hashString)
		if err != nil {
			contextLogger.Error("error key not found in DB")
			contextLogger.Infof("Creating an item in DB with hash val")
			createItem(hashString, resp.amount)
			sendEmail = true
		}

		if dbItem != nil {
			contextLogger.Infof("Found item in DB by hash value")
			amount = dbItem.CurrencyValue
		} else {
			contextLogger.Infof("trying to retrieve value from DB after create")
			record, err := getItem(hashString)
			if err != nil {
				contextLogger.Infof("Failed to get the rec from dynamo. This is unusual !!")
				panic(fmt.Sprintf("Failed to get the rec from DB, %v", err))
			}
			amount = record.CurrencyValue
		}

		if thresholdExceedsPercentVal(resp.amount, amount) {
			contextLogger.Infof("FX Alert threshold diff is greater than 30%")
			sendEmail = true
		}

	} else {
		fmt.Printf("Current FX amount %v", resp.amount)
	}

	if sendEmail {
		contextLogger.Infof("Attempting to send email notification")
		err := sesSendEmail(ctx, resp.amount)
		if err != nil {
			return errors.New("error when sending email")
		}
	}

	return nil
}

func thresholdExceedsPercentVal(currentVal, existingVal float64) bool {
	fmt.Println("Inside threshold func to check if threshold is greater than set percentage i.e ", thresholdPercentage)
	diff := math.Abs(float64(currentVal - existingVal))
	delta := (diff / float64(existingVal)) * 100
	return delta > thresholdPercentage
}

func createItem(hash string, amount float64) {
	fmt.Println("Inside the dynamo create item func")
	svc := dynamodb.New(session.New(aws.NewConfig().WithRegion(awsRegion)))
	expires := time.Now().Add(time.Duration(32400) * time.Second).Unix()
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
	svc := dynamodb.New(session.New(aws.NewConfig().WithRegion(awsRegion)))

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
		return nil, fmt.Errorf("Item not found")
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
	h.Write([]byte(localTime))
	return h.Sum32()
}

func sesSendEmail(ctx context.Context, amount float64) error {
	contextLogger := log.WithContext(ctx)
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
		contextLogger.WithError(err).Error("error when sending email")
		return err
	}
	return nil
}

func getExchangeRate(ctx context.Context) (*ExchangeResponse, error) {
	var httpClient = &http.Client{}
	contextLogger := log.WithContext(ctx)

	httpRequest, err := http.NewRequest(http.MethodGet, buildCurrencyExchangeEndpoint(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		contextLogger.WithError(err).Errorf("there was an error calling the currency exchange API. %v", err)
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			fmt.Println("Error when closing:", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		contextLogger.Infof("status returned from currency exchange service %s", resp.Status)
		return nil, fmt.Errorf("currency exchange service returned status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		contextLogger.WithError(err).Errorf("error reading currency exchange service data resp body (%s)", err)
		return nil, err
	}

	response := &ExchangeResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		contextLogger.WithError(err).Errorf("there was an error un marshalling the currency exchange API resp. %v", err)
		return nil, err
	}

	return response, nil
}

func unMarshallExchangeRate(resp *ExchangeResponse) *Response {
	var fromCurrency = fromCurrency
	var toCurrency = toCurrency
	var conversionMultiple float64
	var exchangeRate float64
	if strings.EqualFold(fromCurrency, "USD") {
		exchangeRate = getRateForCurrency(resp.Rates, toCurrency)
		conversionMultiple = exchangeRate
	} else if strings.EqualFold(toCurrency, "USD") {
		exchangeRate = getRateForCurrency(resp.Rates, fromCurrency)
		conversionMultiple = float64(1) / exchangeRate
	} else {
		// FromCurrency to USD and then USD to toCurrency
		exchangeRate = getRateForCurrency(resp.Rates, toCurrency)
		usdToFromCurrency := getRateForCurrency(resp.Rates, fromCurrency)
		toCurrencyToUSD := float64(1) / exchangeRate
		foreignCurrencyFactor := float64(1) / usdToFromCurrency
		conversionMultiple = foreignCurrencyFactor / toCurrencyToUSD
	}

	return &Response{
		amount: conversionMultiple,
	}
}

func getRateForCurrency(rates map[string]interface{}, currency string) float64 {
	var exchangeRate float64
	for key, rate := range rates {
		if strings.EqualFold(key, currency) {
			exchangeRate = rate.(float64)
			break
		}
	}
	return exchangeRate
}

func buildCurrencyExchangeEndpoint() string {
	return "https://openexchangerates.org/api/latest.json" + "?app_id=" + appID
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
