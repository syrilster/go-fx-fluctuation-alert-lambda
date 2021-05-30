package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/syrilster/go-fx-fluctuation-alert-lambda/fxTrigger"
)

func main() {
	lambda.Start(fxTrigger.Handler)
}

//func init() {
//	var err error
//	toEmail = os.Getenv("TO_EMAIL")
//	fromCurrency = os.Getenv("FROM_CURRENCY")
//	toCurrency = os.Getenv("TO_CURRENCY")
//	if thresholdPercentage, err = strconv.ParseFloat(os.Getenv("THRESHOLD_PERCENT"), 64); err != nil {
//		fmt.Println("error while loading env var THRESHOLD_PERCENT ", err)
//	}
//
//	if currLowerBound, err = strconv.ParseFloat(os.Getenv("LOWER_BOUND"), 64); err != nil {
//		fmt.Println("error while loading env var LOWER_BOUND ", err)
//	}
//
//	if currUpperBound, err = strconv.ParseFloat(os.Getenv("UPPER_BOUND"), 64); err != nil {
//		fmt.Println("error while loading env var UPPER_BOUND ", err)
//	}
//
//	emailClient = ses.New(session.New(), aws.NewConfig().WithRegion(awsRegion))
//}
