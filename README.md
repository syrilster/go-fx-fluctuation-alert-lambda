[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=syrilster_go-fx-fluctuation-alert-lambda&metric=alert_status)](https://sonarcloud.io/dashboard?id=syrilster_go-fx-fluctuation-alert-lambda)

# fx-fluctuation-alert-lambda
* A lambda function to get the currency exchange rate from the API provided by openexchangerates.org and sends an email if the FX price changes as per a preset threshold. (Can be changed form the lamnbda env vars)
* This is run as a cloud watch scheduled event running every hour from 7AM to 06PM.
* To avoid multiple emails during the day once the FX rate threshold is met, we add a dynamodb entry with a TTL of 10 hours with the currency value and hash of the current date as the primary key. 
* Subsequent emails will be sent only if the FX rate changes to a certain percent within a day. (Example increase of 50 cents)

# Uploading lambda to AWS
* Execute these commands in root folder:
  ```
  GOOS=linux GOARCH=amd64 go build -o fx_alert
  zip main.zip fx_alert prod.yaml config/prod.yaml
  ```
