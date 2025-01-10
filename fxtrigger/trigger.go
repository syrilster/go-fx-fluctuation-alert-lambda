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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/rs/zerolog/log"

	"github.com/syrilster/go-fx-fluctuation-alert-lambda/exchange"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/http"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/ses"
	dynamo "github.com/syrilster/go-fx-fluctuation-alert-lambda/pkg/store"
)

const (
	awsRegion     = "ap-south-1"
	configPathKey = "CONFIG_PATH"
	lowerBound    = "LOWER_BOUND"
	upperBound    = "UPPER_BOUND"
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

type DBService struct {
	store dynamo.CurrencySaver
}

var fromCurrency string
var toCurrency string
var dbAmount float32

// NewDBService is accepting interface here
func NewDBService(s dynamo.CurrencySaver) *DBService {
	return &DBService{
		store: s,
	}
}

// Handler func for lambda
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

	if cfg.LowerBound, err = strconv.ParseFloat(os.Getenv(lowerBound), 32); err != nil {
		return errors.New(fmt.Sprint("error while loading env var LOWER_BOUND ", err))
	}

	if cfg.UpperBound, err = strconv.ParseFloat(os.Getenv(upperBound), 32); err != nil {
		return errors.New(fmt.Sprint("error while loading env var UPPER_BOUND ", err))
	}

	_, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		log.Fatal().Msgf("failed to load AWS configuration: %v", err)
		return err
	}

	currencyStore := dynamo.NewCurrencyStore(cfg.FXTableName, dynamodb.New(dynamodb.Options{Region: awsRegion}))

	sesOptions := sesv2.Options{
		Region: awsRegion,
	}
	sesClient, err := ses.New(sesOptions)
	if err != nil {
		log.Fatal().Msgf("failed to create SES client: %v", err)
		return err
	}

	exchangeClient := exchange.NewClient(cfg.ExchangeEndpoint, http.New(), cfg.AppID)
	req := exchange.Request{
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
	}

	return process(ctx, cfg, currencyStore, sesClient, exchangeClient, req)
}

func process(ctx context.Context, cfg *Config, store *dynamo.CurrencyStore, ses *ses.Client, eClient exchange.ClientInterface, request exchange.Request) error {
	ctxLogger := log.Ctx(ctx)

	log.Print("Calling exchange rate API")
	fxAmount, err := eClient.GetExchangeRate(ctx, request)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Error when getting the exchange rate")
		return errors.New("error when getting the exchange rate")
	}
	log.Printf("exchange rate API returned fx rate: %f", fxAmount)

	sendEmail, err := checkThresholdSatisfied(ctx, store, fxAmount, float32(cfg.LowerBound), float32(cfg.UpperBound), cfg.ThresholdPercent)
	if err != nil {
		return fmt.Errorf("error when checking threshold satisfied: %v", err)
	}

	if sendEmail {
		log.Print("Attempting to send email notification")
		err := sesSendEmail(ses, fxAmount, cfg.ToEmail)
		if err != nil {
			return fmt.Errorf("error when sending email: %v", err)
		}
	} else {
		log.Print("FX Alert threshold not met")
		log.Printf("Current FX rate %v", fxAmount)
	}

	return nil
}

func checkThresholdSatisfied(ctx context.Context, store *dynamo.CurrencyStore, fxAmount, lowerBound, upperBound float32, thresholdPercent float64) (sendEmail bool, err error) {
	ctxLogger := log.Ctx(ctx)
	if fxAmount >= upperBound || fxAmount <= lowerBound {
		log.Print("FX threshold satisfied")
		log.Printf("Current FX rate %v", fxAmount)
		if fxAmount <= lowerBound {
			emailText = "LOW"
		}

		hashString := fmt.Sprint(hash())
		ctxLogger.Info().Msgf("computed hash is %v", hashString)
		dbService := NewDBService(store)
		dbItem, err := dbService.getItem(hashString)
		if err != nil {
			ctxLogger.Error().Err(err).Msg("key not found in DynamoDB")
			log.Print("Creating an item in Dynamo with computed hash")
			err := dbService.createItem(hashString, fxAmount)
			if err != nil {
				return false, err
			}
			sendEmail = true
			dbAmount = fxAmount
		}

		if dbItem != nil {
			log.Printf("Found item in DB by hash value: %s", hashString)
			dbAmount = dbItem.CurrencyValue
		}

		if thresholdExceedsPercentVal(thresholdPercent, fxAmount, dbAmount) {
			sendEmail = true
		}
	}
	return sendEmail, nil
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

func (d *DBService) createItem(hash string, amount float32) error {
	expires := getExpiryTime()
	rec := dynamo.Item{
		HashString:    hash,
		CurrencyValue: amount,
		Expires:       expires,
	}

	err := d.store.CreateItem(rec)
	if err != nil {
		log.Error().Err(err).Msg("dynamo create item error")
		return err
	}

	return nil
}

func (d *DBService) getItem(hash string) (*dynamo.Item, error) {
	resp, err := d.store.GetItem(hash)
	if err != nil {
		log.Error().Err(err).Msg("dynamo getItem error")
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return resp, nil
}

func sesSendEmail(sesClient *ses.Client, amount float32, toEmail string) error {
	// Construct the SendEmailInput
	emailParams := &sesv2.SendEmailInput{
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(fromCurrency + " to " + toCurrency + " Alert"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data: aws.String(fromCurrency + " to " + toCurrency + " value is " + emailText + ". Current value is " + fmt.Sprintf("%f", amount)),
					},
				},
			},
		},
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		FromEmailAddress: aws.String(toEmail),
	}

	// Send the email
	_, err := sesClient.SendEmail(context.TODO(), emailParams)
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
