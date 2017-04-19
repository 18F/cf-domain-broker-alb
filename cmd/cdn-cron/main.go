package main

import (
	"os"
	"os/signal"

	"code.cloudfoundry.org/lager"
	"github.com/robfig/cron"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/18F/cf-cdn-service-broker/config"
	"github.com/18F/cf-cdn-service-broker/models"
	"github.com/18F/cf-cdn-service-broker/utils"
)

func main() {
	logger := lager.NewLogger("cdn-cron")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	settings, err := config.NewSettings()
	if err != nil {
		logger.Fatal("new-settings", err)
	}

	session := session.New(aws.NewConfig().WithRegion(settings.AwsDefaultRegion))

	client, err := utils.NewClient(settings, s3.New(session))
	if err != nil {
		logger.Fatal("acme-client", err)
	}

	db, err := config.Connect(settings)
	if err != nil {
		logger.Fatal("connect", err)
	}

	manager := models.NewManager(
		logger,
		&utils.Iam{settings, iam.New(session)},
		&utils.Distribution{settings, cloudfront.New(session)},
		client,
		db,
	)

	c := cron.New()

	c.AddFunc(settings.Schedule, func() {
		logger.Info("Running renew")
		manager.RenewAll()
	})

	c.AddFunc(settings.Schedule, func() {
		logger.Info("Running cert cleanup")
		manager.DeleteOrphanedCerts()
	})

	logger.Info("Starting cron")
	c.Start()

	waitForExit()
}

func waitForExit() os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return <-c
}
