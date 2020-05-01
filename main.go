package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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

var toEmail string
var appId string
var fromCurrency string
var toCurrency string
var emailClient *ses.SES

func init() {
	toEmail = os.Getenv("TO_EMAIL")
	appId = os.Getenv("APP_ID")
	fromCurrency = os.Getenv("FROM_CURRENCY")
	toCurrency = os.Getenv("TO_CURRENCY")

	emailClient = ses.New(session.New(), aws.NewConfig().WithRegion("ap-south-1"))
}

//Handler func for lambda
func Handler(ctx context.Context, request CustomEvent) error {
	contextLogger := log.WithContext(ctx)
	contextLogger.Infof("Inside the lambda handler func")

	var sendEmail bool

	exchangeResponse, err := GetExchangeRate(ctx)
	if err != nil {
		contextLogger.WithError(err).Error("error when getting the exchange rate")
		return errors.New("error when getting the exchange rate")
	}

	resp := unMarshallExchangeRate(exchangeResponse)

	if resp.amount >= 48 || resp.amount <= 48 {
		contextLogger.Infof("FX Alert threshold satisfied")
		sendEmail = true
	} else {
		fmt.Printf("Current FX amount %v", resp.amount)
	}

	if sendEmail {
		contextLogger.Infof("Attempting to send email notification")
		err := SesSendEmail(ctx, resp.amount)
		if err != nil {
			return errors.New("error when sending email")
		}
	}

	return nil
}

func SesSendEmail(ctx context.Context, amount float64) error {
	contextLogger := log.WithContext(ctx)
	emailParams := &ses.SendEmailInput{
		Message: &ses.Message{
			Subject: &ses.Content{
				Data: aws.String(fromCurrency + " to " + toCurrency + " Alert"),
			},
			Body: &ses.Body{
				Text: &ses.Content{
					Data: aws.String(fromCurrency + " to " + toCurrency + " value is HIGH. Current value is " + fmt.Sprintf("%f", amount)),
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

func GetExchangeRate(ctx context.Context) (*ExchangeResponse, error) {
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
	return "https://openexchangerates.org/api/latest.json" + "?app_id=" + appId
}
