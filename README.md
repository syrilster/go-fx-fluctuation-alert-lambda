# fx-fluctuation-alert-lambda
* A lambda function to get the currency exchange rate from openexchangerates.org and send an email if the FX price changes as per desired threshold. 
* This is run as a cloud watch scheduled event running every hour from 8AM to 10PM.
* To avoid multiple emails during the day once the FX rate threshold is met, we add a dynamodb entry with a TTL of 10 hours with the currency value and hash of the current date as the primary key. 
* Subsequent emails will be sent only if the FX rate changes to a certain percent within a day. (Example increase of 50 cents)

# Uploading lambda to AWS
* Execute these commands in root folder:
  ```
  GOOS=linux GOARCH=amd64 go build -o main main.go
  zip main.zip main
  ```
