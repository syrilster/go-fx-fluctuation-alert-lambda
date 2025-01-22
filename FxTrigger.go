package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/internal/fxtrigger"
)

func main() {
	lambda.Start(fxtrigger.Handler)
}
