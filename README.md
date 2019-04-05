# wtm-demo
Code and documentation used for demo at WTM Barcelona 2019 conference

## S3 buckets
Create two S3 buckets
* for saving modified images
* for saving combined image

## Lambda functions
### Prerequisits
- [Golang installed](https://golang.org/doc/install)
- [dep installed](https://golang.github.io/dep/docs/installation.html)

### Build

* Install dependency for both lambda functions
```
cd lambdaModify/ && dep ensure
cd lambdaCombine/ && dep ensure
```
* Compile lambda functions
```
GOOS=linux GOARCH=amd64 go build -o bin/modify lambdaModify/imageModify.go
GOOS=linux GOARCH=amd64 go build -o bin/combine lambdaCombine/imagesCombine.go
```
* Create a deployment packages
```
zip modify.zip bin/modify
zip combine.zip bin/combine
```

### Create your Lambda fucntions in AWS
* Go to [AWS Lambda console](https://console.aws.amazon.com/lambda/home) and create two Golang functions:
    * `imageModify`
    * `imageCombine`
* Upload both functions to AWS Lambda to region of your choice
```
aws lambda update-function-code --function-name imageModify --zip-file fileb://modify.zip --region REGION_NAME
aws lambda update-function-code --function-name imageCombine --zip-file fileb://combine.zip --region REGION_NAME
```
* Update configuration to save images into proper buckets, that you created on the beginning
```
aws lambda update-function-configuration --function-name imageModify --environment Variables="{BUCKET_FOR_SAVING_IMG=_bucket1_}"
aws lambda update-function-configuration --function-name imageCombine --environment Variables="{BUCKET_FOR_SAVING_COMBINED_IMG=_bucket2_}"
```

Make sure that you have proper permissions for Lambda and S3 buckets (Lambda needs to have permissions to write to S3 buckets)

## TODO
* Include diagram
* Specify exact permissions