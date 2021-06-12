package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	chttp "github.com/syrilster/go-fx-fluctuation-alert-lambda/http"
)

type ClientInterface interface {
	GetExchangeRate(ctx context.Context, request Request) (float32, error)
}

func NewClient(endpoint string, h chttp.HTTPClient, appID string) *client {
	return &client{
		URL:         endpoint,
		HttpCommand: h,
		AppID:       appID,
	}
}

type client struct {
	URL         string
	HttpCommand chttp.HTTPClient
	AppID       string
}

func (c *client) GetExchangeRate(ctx context.Context, request Request) (float32, error) {
	ctxLogger := log.Ctx(ctx)

	defaultResp := float32(0)
	httpRequest, err := http.NewRequest(http.MethodGet, c.buildCurrencyExchangeEndpoint(), nil)
	if err != nil {
		return defaultResp, err
	}

	resp, err := c.HttpCommand.Do(httpRequest)
	if err != nil {
		ctxLogger.Error().Err(err).Msgf("there was an error calling the currency exchange API. %v", err)
		return defaultResp, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			fmt.Println("Error when closing:", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("status returned from currency exchange service %s", resp.Status)
		return defaultResp, fmt.Errorf("currency exchange service returned status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctxLogger.Error().Err(err).Msgf("error reading currency exchange service data resp body (%s)", err)
		return defaultResp, err
	}

	r := &Response{}
	if err := json.Unmarshal(body, r); err != nil {
		ctxLogger.Error().Err(err).Msgf("there was an error un marshalling the currency exchange API resp. %v", err)
		return defaultResp, err
	}

	response := unMarshallExchangeRate(r, request)
	return response, nil
}

func unMarshallExchangeRate(resp *Response, req Request) float32 {
	var fromCurrency = req.FromCurrency
	var toCurrency = req.ToCurrency
	var conversionMultiple float32
	var exchangeRate float32
	if strings.EqualFold(fromCurrency, "USD") {
		exchangeRate = getRateForCurrency(resp.Rates, toCurrency)
		conversionMultiple = exchangeRate
	} else if strings.EqualFold(toCurrency, "USD") {
		exchangeRate = getRateForCurrency(resp.Rates, fromCurrency)
		conversionMultiple = float32(1) / exchangeRate
	} else {
		// FromCurrency to USD and then USD to toCurrency
		exchangeRate = getRateForCurrency(resp.Rates, toCurrency)
		usdToFromCurrency := getRateForCurrency(resp.Rates, fromCurrency)
		toCurrencyToUSD := float32(1) / exchangeRate
		foreignCurrencyFactor := float32(1) / usdToFromCurrency
		conversionMultiple = foreignCurrencyFactor / toCurrencyToUSD
	}

	return conversionMultiple
}

func getRateForCurrency(rates map[string]interface{}, currency string) float32 {
	var exchangeRate float64
	for key, rate := range rates {
		if strings.EqualFold(key, currency) {
			exchangeRate = rate.(float64)
			break
		}
	}
	return float32(exchangeRate)
}

func (c *client) buildCurrencyExchangeEndpoint() (endpoint string) {
	return c.URL + "?app_id=" + c.AppID
}
