package exchange

type Request struct {
	FromCurrency string `json:"from"`
	ToCurrency   string `json:"to"`
}

type Response struct {
	Base  string                 `json:"base"`
	Rates map[string]interface{} `json:"rates"`
}
