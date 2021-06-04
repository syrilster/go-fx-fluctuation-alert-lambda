package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/fxtrigger"
)

func main() {
	lambda.Start(fxtrigger.Handler)
}
