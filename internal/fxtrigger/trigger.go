package fxtrigger

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/exchange"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/ses"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/store"
)

const (
	awsRegion     = "ap-south-1"
	configPathKey = "CONFIG_PATH"
	lowerBound    = "LOWER_BOUND"
	upperBound    = "UPPER_BOUND"
	loggerKey     = "logger"
)

var emailText = "HIGH"
var fromCurrency string
var toCurrency string
var dbAmount float32

type CustomEvent struct {
	Name string `json:"name"`
}

type ExchangeResponse struct {
	From   string `json:"from"`
	To     string `json:"to"`
	FXRate string `json:"conversion_multiple"`
}

type DBService struct {
	store store.CurrencySaver
}

// NewDBService is accepting interface here
func NewDBService(s store.CurrencySaver) *DBService {
	return &DBService{
		store: s,
	}
}

// Handler func for lambda
func Handler(ctx context.Context, request CustomEvent) error {
	var err error
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Attach logger to the context
	ctxLogger := context.WithValue(ctx, loggerKey, log)
	log.Info("Inside the lambda handler", slog.String("date", getLocalTime()))

	cfgPath := os.Getenv(configPathKey)

	log.Info(fmt.Sprintf("Loading config values from path: %s", cfgPath))
	var c Config
	cfg := c.getConfig(cfgPath)

	log.Info("Config Loaded Successfully")
	cfg.ToEmail = os.Getenv("TO_EMAIL")
	cfg.AppID = os.Getenv("APP_ID")
	fromCurrency = os.Getenv("FROM_CURRENCY")
	toCurrency = os.Getenv("TO_CURRENCY")

	if cfg.LowerBound, err = strconv.ParseFloat(os.Getenv(lowerBound), 32); err != nil {
		return errors.New(fmt.Sprint("failed loading env var LOWER_BOUND ", err))
	}

	if cfg.UpperBound, err = strconv.ParseFloat(os.Getenv(upperBound), 32); err != nil {
		return errors.New(fmt.Sprint("failed loading env var UPPER_BOUND ", err))
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		log.Error("failed to load AWS configuration", slog.Any("error", err))
		return err
	}

	currencyStore := store.NewCurrencyStore(cfg.FXTableName, dynamodb.NewFromConfig(awsCfg))
	sesClient, err := ses.New(awsCfg)
	if err != nil {
		log.Error("failed to create SES client", slog.Any("error", err))
		return err
	}

	exchangeClient := exchange.NewClient(cfg.ExchangeEndpoint, exchange.New(), cfg.AppID)
	req := exchange.Request{
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
	}

	return process(ctxLogger, cfg, currencyStore, sesClient, exchangeClient, req)
}

func process(ctx context.Context, cfg *Config, store *store.CurrencyStore, ses *ses.Client, eClient exchange.ClientInterface, request exchange.Request) error {
	log := loggerFromContext(ctx)

	log.Info("Calling exchange rate API")
	fxAmount, err := eClient.GetExchangeRate(ctx, request)
	if err != nil {
		log.Error("Error when getting the exchange rate", slog.Any("error", err))
		return fmt.Errorf("failed to get the exchange rate: %v", err)
	}

	log.Info(fmt.Sprintf("exchange rate API returned fx rate: %f", fxAmount))
	sendEmail, err := checkThresholdSatisfied(ctx, store, fxAmount, float32(cfg.LowerBound), float32(cfg.UpperBound), cfg.ThresholdPercent)
	if err != nil {
		return fmt.Errorf("failed to check threshold: %v", err)
	}

	if sendEmail {
		log.Info("Send email notification")
		err := sesSendEmail(ses, fxAmount, cfg.ToEmail)
		if err != nil {
			return fmt.Errorf("failed to send email: %v", err)
		}
	} else {
		log.Info("FX alert threshold not met")
	}

	return nil
}

func checkThresholdSatisfied(ctx context.Context, store *store.CurrencyStore, fxAmount, lowerBound, upperBound float32, thresholdPercent float64) (sendEmail bool, err error) {
	log := loggerFromContext(ctx)
	if fxAmount >= upperBound || fxAmount <= lowerBound {
		log.Info("FX threshold satisfied")
		if fxAmount <= lowerBound {
			emailText = "LOW"
		}

		hashString := fmt.Sprint(hash())
		log.Info(fmt.Sprintf("Computed hash value is: %s", hashString))
		dbService := NewDBService(store)
		dbItem, err := dbService.getItem(ctx, hashString)
		if err != nil {
			log.Error("key not found in DynamoDB", slog.Any("error", err))
			log.Info("Creating an item in Dynamo with computed hash")
			err := dbService.createItem(ctx, hashString, fxAmount)
			if err != nil {
				return false, err
			}
			sendEmail = true
			dbAmount = fxAmount
		}

		if dbItem != nil {
			log.Info(fmt.Sprintf("Found item in DB by hash value: %s", hashString))
			dbAmount = dbItem.CurrencyValue
		}

		if thresholdExceedsPercentVal(ctx, thresholdPercent, fxAmount, dbAmount) {
			sendEmail = true
		}
	}
	return sendEmail, nil
}

func thresholdExceedsPercentVal(ctx context.Context, thresholdPercent float64, currentVal, existingVal float32) bool {
	log := loggerFromContext(ctx)
	if currentVal == existingVal {
		return false
	}

	log.Info(fmt.Sprintf("checking if thresholdPercent is greater than set percentage: %f", thresholdPercent))
	diff := math.Abs(float64(currentVal) - float64(existingVal))
	delta := (diff / float64(existingVal)) * 100
	log.Info(fmt.Sprintf("percent diff with previous value is: %f", delta))
	return delta > thresholdPercent
}

func (d *DBService) createItem(ctx context.Context, hash string, amount float32) error {
	log := loggerFromContext(ctx)
	expires := getExpiryTime()
	rec := store.Item{
		HashString:    hash,
		CurrencyValue: amount,
		Expires:       expires,
	}

	err := d.store.CreateItem(rec)
	if err != nil {
		log.Error("dynamo create item error", slog.Any("error", err))
		return err
	}

	return nil
}

func (d *DBService) getItem(ctx context.Context, hash string) (*store.Item, error) {
	log := loggerFromContext(ctx)
	resp, err := d.store.GetItem(hash)
	if err != nil {
		log.Error("dynamo getItem error", slog.Any("error", err))
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

func loggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		// Return a default logger if none is found
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return logger
}
