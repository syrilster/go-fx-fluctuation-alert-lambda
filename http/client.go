package http

import "net/http"

type CustomHttp interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type Client struct {
	client *http.Client
}

func New() *Client {
	return &Client{client: http.DefaultClient}
}

func (c Client) Do(req *http.Request) (resp *http.Response, err error) {
	return c.client.Do(req)
}
