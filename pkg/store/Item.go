package store

type Item struct {
	HashString    string  `json:"hash" dynamodbav:"hash"`
	CurrencyValue float32 `json:"currency_value" dynamodbav:"currency_value"`
	Expires       int64   `json:"expires_at" dynamodbav:"expires_at"`
}
