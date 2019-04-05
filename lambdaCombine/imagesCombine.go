package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/imaging"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"sort"
	"time"
)

type item struct {
	name         string
	lastModified time.Time
}

func downloadAndDecode(s3dl *s3manager.Downloader, bucket, key string) (image.Image, error) {
	buff := &aws.WriteAtBuffer{}
	_, err := s3dl.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("Could not download from S3: %v", err)
	}

	// Decoding image
	imgBytes := buff.Bytes()
	img, err := jpeg.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("jpg decode error: %v", err)
	}
	return img, nil
}

func imageCombine(images []image.Image) image.Image {
	var width, height int

	switch {
	case len(images) <= 25:
		width = 2100
		height = 2100
	case len(images) > 25 && len(images) <= 50:
		width = 2100
		height = 4200
	case len(images) > 50:
		width = 2100
		height = 6300
	}

	dst := imaging.New(width, height, color.NRGBA{0, 0, 0, 0})

	d := 5 // number of pictures in a row

	for k, v := range images {
		y, x := k/d, k%d
		pos1, pos2 := int((width/d)*x), int((width/d)*y)
		dst = imaging.Paste(dst, v, image.Pt(pos1, pos2))
	}
	return dst
}

func handler(ctx context.Context, event events.S3Event) error {
	for _, record := range event.Records {

		bucket := record.S3.Bucket.Name
		key := record.S3.Object.Key
		log.Printf("Using bucket: %s and key: %s", bucket, key)

		// initialize session
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String(record.AWSRegion)},
		)
		if err != nil {
			return fmt.Errorf("cannot initialize session: %v", err)
		}

		// list objects in bucket
		svc := s3.New(sess)
		resp, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(bucket)})
		if err != nil {
			return fmt.Errorf("unable to list items in bucket %q, %v", bucket, err)
		}
		if len(resp.Contents) > 75 {
			return fmt.Errorf("stop processing, too many images to process, limit 75, got: %d", len(resp.Contents))
		}

		s3Items := make([]item, len(resp.Contents))
		for i, o := range resp.Contents {
			s3Items[i] = item{name: *o.Key, lastModified: *o.LastModified}
		}

		sort.Slice(s3Items, func(i, j int) bool { return s3Items[i].lastModified.Before(s3Items[j].lastModified) })

		// download objects from the bucket
		s3dl := s3manager.NewDownloader(sess)
		images := make([]image.Image, 0)
		for _, o := range s3Items {
			img, err := downloadAndDecode(s3dl, bucket, o.name)
			if err != nil {
				log.Printf("Cannot download %s from %s, %v", o.name, bucket, err)
				continue
			}
			images = append(images, img)
		}

		// create combined image
		newImg := imageCombine(images)
		buf := new(bytes.Buffer)
		err = png.Encode(buf, newImg)
		if err != nil {
			return fmt.Errorf("encoding error: %v", err)
		}

		log.Println("Starting upload to S3")
		bucketForSave := os.Getenv("BUCKET_FOR_SAVING_COMBINED_IMG")
		if bucketForSave == "" {
			return errors.New("cannot find bucket name, specify BUCKET_FOR_SAVING_COMBINED_IMG env")
		}
		uploader := s3manager.NewUploader(sess)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Body:        bytes.NewReader(buf.Bytes()),
			Bucket:      aws.String(bucketForSave),
			Key:         aws.String("combined.png"),
			ContentType: aws.String("image/png"),
		})
		if err != nil {
			return fmt.Errorf("Failed to upload: %v", err)
		}
		log.Printf("Successfully uploaded to: %v", result.Location)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
