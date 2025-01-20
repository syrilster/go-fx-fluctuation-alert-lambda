[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=syrilster_go-fx-fluctuation-alert-lambda&metric=alert_status)](https://sonarcloud.io/dashboard?id=syrilster_go-fx-fluctuation-alert-lambda)

# fx-fluctuation-alert-lambda

- A Lambda function designed to fetch currency exchange rates from the API provided by openexchangerates.org and send an email alert if the FX rate fluctuates beyond a preset threshold. (The threshold can be configured using Lambda environment variables.)
- This function is triggered as a CloudWatch scheduled event, running hourly from 7:00 AM to 6:00 PM.
- To prevent sending multiple emails once the FX rate threshold is met, a DynamoDB entry is created with a 10-hour TTL. This entry stores the currency value and a hash of the current date as the primary key.
- Subsequent alerts are sent only if the FX rate changes by a specified percentage within the same day (e.g., an increase of 50 cents).

# Uploading lambda to AWS
* GHA runs on master is available to deploy zip to AWS lambda automatically.
* Execute these commands in root folder:
  ```
  GOOS=linux GOARCH=amd64 go build -o fx_alert
  zip main.zip fx_alert prod.yaml config/prod.yaml
  ```
