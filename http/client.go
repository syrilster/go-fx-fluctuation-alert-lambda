package http

import "net/http"

type Client interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type HTTP struct {
	client *http.Client
}

func New() *HTTP {
	return &HTTP{client: http.DefaultClient}
}

func (c HTTP) Do(req *http.Request) (resp *http.Response, err error) {
	return c.client.Do(req)
}
