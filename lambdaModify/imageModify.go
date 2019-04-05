package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/imaging"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"regexp"
	"strings"
)

func imageModify(input image.Image, operation string) (image.Image, error) {

	if input.Bounds().Dx() <= input.Bounds().Dy() {
		input = imaging.CropCenter(input, input.Bounds().Dx(), input.Bounds().Dx())
	} else {
		input = imaging.CropCenter(input, input.Bounds().Dy(), input.Bounds().Dy())
	}

	input = imaging.Resize(input, 400, 400, imaging.Lanczos)

	switch operation {
	case "blur":
		return imaging.Blur(input, 1.5), nil
	case "grayscale":
		return imaging.Grayscale(input), nil
	case "invert":
		return imaging.Invert(input), nil
	case "adjustSaturation":
		return imaging.AdjustSaturation(input, 40), nil
	case "all":
		resized := imaging.Resize(input, 200, 0, imaging.Lanczos)
		output := imaging.New(400, 400, color.NRGBA{0, 0, 0, 0})
		output = imaging.Paste(output, imaging.Blur(resized, 1.5), image.Pt(0, 0))
		output = imaging.Paste(output, imaging.Grayscale(resized), image.Pt(0, 200))
		output = imaging.Paste(output, imaging.Invert(resized), image.Pt(200, 0))
		output = imaging.Paste(output, imaging.AdjustSaturation(resized, 40), image.Pt(200, 200))
		return output, nil
	default:
		return nil, fmt.Errorf("non expected operation: %s", operation)
	}
}

type Event struct {
	Operation   string `json:"operation"`
	Base64Image string `json:"base64Image"`
	ImgName     string `json:"imgName"`
}

func handler(ctx context.Context, event Event) (string, error) {

	log.Printf("Starting processing image: %s", event.ImgName)

	re := regexp.MustCompile(`(.*)\.(jpg|jpeg|png|PNG|JPG|JPEG)`)
	match := re.FindStringSubmatch(event.ImgName)
	if len(match) != 3 {
		return "", errors.New("cannot extract image extention")
	}
	imgNameNoExt := match[1]
	imgExtension := match[2]

	log.Println("Decoding to Image")
	imgBytes, err := base64.StdEncoding.DecodeString(event.Base64Image)
	if err != nil {
		return "", fmt.Errorf("cannot decode 64bytes image string, incorrect input image, err: %v", err)
	}
	var img image.Image
	switch strings.ToLower(imgExtension) {
	case "png":
		img, err = png.Decode(bytes.NewReader(imgBytes))
	case "jpeg", "jpg":
		img, err = jpeg.Decode(bytes.NewReader(imgBytes))
	default:
		return "", fmt.Errorf("non expected image extension, got: %s, expected: png, jpg, jpeg", imgExtension)
	}
	if err != nil {
		return "", fmt.Errorf("%s decode error: %v", imgExtension, err)
	}

	log.Println("Modifying image")
	newImg, err := imageModify(img, event.Operation)
	if err != nil {
		return "", fmt.Errorf("cannot modify image, %v", err)
	}

	log.Println("Encoding image to jpg")
	imgExtension = "jpg"
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, newImg, nil)
	if err != nil {
		return "", fmt.Errorf("%s encoding error: %v", imgExtension, err)
	}
	encodedString := base64.StdEncoding.EncodeToString(buf.Bytes())

	log.Println("Starting upload to S3")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION"))},
	)
	if err != nil {
		log.Printf("Cannot connect to S3 bucket, image not uploaded")
		return encodedString, nil
	}

	key := fmt.Sprintf("%s-modified.%s", strings.TrimSpace(imgNameNoExt), imgExtension)
	bucketName := os.Getenv("BUCKET_FOR_SAVING_IMG")
	if bucketName == "" {
		log.Println("Cannot find bucket name, specify BUCKET_FOR_SAVING_IMG env")
		return encodedString, nil
	}
	uploader := s3manager.NewUploader(sess)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Body:   bytes.NewReader(buf.Bytes()),
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Printf("Failed to upload: %v", err)
		return encodedString, nil
	}
	log.Printf("Successfully uploaded to: %v", result.Location)

	return encodedString, nil
}

func main() {
	lambda.Start(handler)
}
