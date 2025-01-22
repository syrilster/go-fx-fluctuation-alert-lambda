package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct {
	doFn func(*http.Request) (*http.Response, error)
}

func (m *mockClient) Do(req *http.Request) (resp *http.Response, err error) {
	return m.doFn(req)
}

func TestGetExchangeRate(t *testing.T) {
	m := mockClient{}
	c := NewClient("", &m, "")

	t.Run("Failed to send request", func(t *testing.T) {
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return nil, fmt.Errorf("unable to send request")
		}

		_, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "AUD",
			ToCurrency:   "INR",
		})
		assert.EqualError(t, err, "unable to send request")
	})

	t.Run("Request is not successful", func(t *testing.T) {
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return &http.Response{
				Status:     "StatusInternalServerError",
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		}

		_, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "AUD",
			ToCurrency:   "INR",
		})
		assert.EqualError(t, err, "currency exchange service returned status: StatusInternalServerError")
	})

	t.Run("Success with empty response", func(t *testing.T) {
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		}

		resp, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "AUD",
			ToCurrency:   "INR",
		})
		assert.Equal(t, float32(0), resp)
		require.Error(t, err)
	})

	t.Run("SuccessNonUSD", func(t *testing.T) {
		rates := map[string]interface{}{
			"AED": 3.672538,
			"AUD": 1.390866,
			"ALL": 125.716501,
		}

		obj := Response{
			Base:  "USD",
			Rates: rates,
		}
		rJson, _ := json.Marshal(obj)
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rJson)),
			}, nil
		}

		resp, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "AUD",
			ToCurrency:   "AED",
		})
		assert.Equal(t, float32(2.6404686), resp)
		require.NoError(t, err)
	})

	t.Run("SuccessFromCurrencyUSD", func(t *testing.T) {
		rates := map[string]interface{}{
			"AED": 3.672538,
			"AUD": 1.390866,
			"ALL": 125.716501,
			"USD": 1,
		}

		obj := Response{
			Base:  "USD",
			Rates: rates,
		}
		rJson, _ := json.Marshal(obj)
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rJson)),
			}, nil
		}

		resp, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "USD",
			ToCurrency:   "AED",
		})
		assert.Equal(t, float32(3.672538), resp)
		require.NoError(t, err)
	})

	t.Run("SuccessToCurrencyUSD", func(t *testing.T) {
		rates := map[string]interface{}{
			"AED": 3.672538,
			"AUD": 1.390866,
			"ALL": 125.716501,
			"USD": 1,
		}

		obj := Response{
			Base:  "USD",
			Rates: rates,
		}
		rJson, _ := json.Marshal(obj)
		m.doFn = func(request *http.Request) (response *http.Response, e error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(rJson)),
			}, nil
		}

		resp, err := c.GetExchangeRate(context.Background(), Request{
			FromCurrency: "AED",
			ToCurrency:   "USD",
		})
		assert.Equal(t, float32(0.27229124), resp)
		require.NoError(t, err)
	})
}
