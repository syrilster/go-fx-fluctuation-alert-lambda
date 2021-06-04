package fxtrigger

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	pses "github.com/aws/aws-sdk-go/service/ses"

	"github.com/syrilster/go-fx-fluctuation-alert-lambda/exchange"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/http"
	dynamo "github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/dynamodb"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/ses"
)

const (
	awsRegion     = "ap-south-1"
	configPathKey = "CONFIG_PATH"
)

var emailText = "HIGH"

type CustomEvent struct {
	Name string `json:"name"`
}

type ExchangeResponse struct {
	From   string `json:"from"`
	To     string `json:"to"`
	FXRate string `json:"conversion_multiple"`
}

type Item struct {
	HashString    string  `json:"hash"`
	CurrencyValue float32 `json:"currency_value"`
	Expires       int64   `json:"expires_at"`
}

var fromCurrency string
var toCurrency string
var dbAmount float32

//Handler func for lambda
func Handler(ctx context.Context, request CustomEvent) error {
	var err error
	contextLogger := log.Ctx(ctx)
	contextLogger.Info().Msgf("Inside the lambda handler at date: %s", getLocalTime())
	contextLogger.Info().Msgf("Event Trigger: %s", request.Name)

	cfgPath := os.Getenv(configPathKey)

	log.Print("Loading Config from path:", configPathKey)
	var c Config
	cfg := c.getConfig(cfgPath)

	log.Print("Config Loaded Successfully")
	cfg.ToEmail = os.Getenv("TO_EMAIL")
	cfg.AppID = os.Getenv("APP_ID")
	fromCurrency = os.Getenv("FROM_CURRENCY")
	toCurrency = os.Getenv("TO_CURRENCY")

	if cfg.LowerBound, err = strconv.ParseFloat(os.Getenv("LOWER_BOUND"), 32); err != nil {
		panic(fmt.Sprint("error while loading env var LOWER_BOUND ", err))
	}

	if cfg.UpperBound, err = strconv.ParseFloat(os.Getenv("UPPER_BOUND"), 32); err != nil {
		panic(fmt.Sprint("error while loading env var UPPER_BOUND ", err))
	}

	dbSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(cfg.AWSRegion),
	}))
	dbClient := dynamo.New(cfg.FXTableName, dynamodb.New(dbSession))

	sesClient, err := ses.New(awsRegion)
	if err != nil {
		return err
	}

	exchangeClient := exchange.NewClient(cfg.ExchangeEndpoint, http.New(), cfg.AppID)
	req := exchange.Request{
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
	}

	return process(ctx, cfg, dbClient, sesClient, exchangeClient, req)
}

func process(ctx context.Context, cfg *Config, store *dynamo.DynamoStore, ses *ses.ClientAdapter, eClient exchange.ClientInterface, request exchange.Request) error {
	var sendEmail bool
	ctxLogger := log.Ctx(ctx)

	log.Print("Calling exchange rate API")
	fxAmount, err := eClient.GetExchangeRate(ctx, request)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Error when getting the exchange rate")
		return errors.New("error when getting the exchange rate")
	}
	log.Printf("exchange rate API returned fx rate: %f", fxAmount)

	if fxAmount >= float32(cfg.UpperBound) || fxAmount <= float32(cfg.LowerBound) {
		log.Print("FX threshold satisfied")
		log.Printf("Current FX rate %v", fxAmount)
		if fxAmount <= float32(cfg.LowerBound) {
			emailText = "LOW"
		}

		hashString := fmt.Sprint(hash())
		ctxLogger.Info().Msgf("computed hash is %v", hashString)
		dbItem, err := getItem(store, hashString)
		if err != nil {
			ctxLogger.Error().Err(err).Msg("key not found in DynamoDB")
			log.Print("Creating an item in Dynamo with computed hash")
			createItem(store, hashString, fxAmount)
			sendEmail = true
			dbAmount = fxAmount
		}

		if dbItem != nil {
			log.Printf("Found item in DB by hash value: %s", hashString)
			dbAmount = dbItem.CurrencyValue
		}

		if thresholdExceedsPercentVal(cfg.ThresholdPercent, fxAmount, dbAmount) {
			sendEmail = true
		}

	} else {
		log.Print("FX Alert threshold not met")
		log.Printf("Current FX rate %v", fxAmount)
	}

	if sendEmail {
		log.Print("Attempting to send email notification")
		err := sesSendEmail(ses, fxAmount, cfg.ToEmail)
		if err != nil {
			return errors.New("error when sending email")
		}
	}

	return nil
}

func thresholdExceedsPercentVal(threshold float64, currentVal, existingVal float32) bool {
	if currentVal == existingVal {
		return false
	}

	log.Printf("Inside threshold func to check if threshold is greater than set percentage: %f", threshold)
	diff := math.Abs(float64(currentVal) - float64(existingVal))
	delta := (diff / float64(existingVal)) * 100
	log.Printf("percent diff with prev value is: %f", delta)
	return delta > threshold
}

func createItem(store *dynamo.DynamoStore, hash string, amount float32) {
	expires := getExpiryTime()
	rec := Item{
		hash,
		amount,
		expires,
	}

	err := dynamo.Create(store.DB, store.TableName, rec)
	if err != nil {
		log.Error().Err(err).Msg("dynamo create item error")
		panic(fmt.Sprintf("dynamo create item error %v", err))
	}
}

func getItem(store *dynamo.DynamoStore, hash string) (*Item, error) {
	item := &Item{}

	err := dynamo.GetItem(store.DB, store.TableName, hash, item)
	if err != nil {
		log.Error().Err(err).Msg("dynamo getItem error")
		return nil, err
	}

	return item, nil
}

func sesSendEmail(ses *ses.ClientAdapter, amount float32, toEmail string) error {
	emailParams := &pses.SendEmailInput{
		Message: &pses.Message{
			Subject: &pses.Content{
				Data: aws.String(fromCurrency + " to " + toCurrency + " Alert"),
			},
			Body: &pses.Body{
				Text: &pses.Content{
					Data: aws.String(fromCurrency + " to " + toCurrency + " value is " + emailText + ". Current value is " + fmt.Sprintf("%f", amount)),
				},
			},
		},
		Destination: &pses.Destination{
			ToAddresses: []*string{aws.String(toEmail)},
		},
		Source: aws.String(toEmail),
	}

	_, err := ses.SendEmail(emailParams)
	if err != nil {
		return err
	}
	return nil
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
