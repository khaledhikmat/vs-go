package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/xerrors"

	"github.com/khaledhikmat/vs-go/mode"
	"github.com/khaledhikmat/vs-go/pipeline"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/inference"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"github.com/khaledhikmat/vs-go/service/orphan"
	"github.com/khaledhikmat/vs-go/service/storage"
	"github.com/khaledhikmat/vs-go/service/vms"
	"github.com/khaledhikmat/vs-go/service/webhook"
)

const (
	// WARNING: this has to be bigger that the mode processor shutdown time
	waitOnShutdown = 8 * time.Second
)

var modeProcessors = map[string]mode.Processor{
	"manager": mode.Manager,
	"monitor": mode.Monitor,
}

func main() {
	rootCtx := context.Background()
	canxCtx, canxFn := context.WithCancel(rootCtx)

	// Hook up a signal handler to cancel the context
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		lgr.Logger.Info(
			"received kill signal",
			slog.Any("signal", sig),
		)
		canxFn()
	}()

	// Load env vars if we are in DEV mode
	if os.Getenv("RUN_TIME_ENV") == "dev" || os.Getenv("RUN_TIME_ENV") == "" {
		lgr.Logger.Info("loading env vars from .env file")
		err := godotenv.Load()
		if err != nil {
			lgr.Logger.Error("error loading .env file", slog.Any("error", xerrors.New(err.Error())))
			panic("error loading .env file")
		}
	}

	modeType := "manager"
	args := os.Args[1:]
	if len(args) > 0 {
		modeType = args[0]
	}

	modeProc, ok := modeProcessors[modeType]
	if !ok {
		lgr.Logger.Error("invalid mode", slog.String("mode", modeType))
		panic("invalid mode")
	}

	// Create the services needed for the mode processor
	// They can be overridden by the mode processor with different implementations
	// Config service
	cfgSvc := config.NewHardCoded()
	// Data service
	dataSvc := data.NewFilesDB(cfgSvc)
	// Orphan service
	orphanSvc := orphan.NewTimed(canxCtx, cfgSvc, dataSvc)
	// storage service
	storageSvc := storage.NewFake(cfgSvc)
	// vms service
	vmsSvc := vms.NewFake(cfgSvc, storageSvc)
	// inference service
	inferenceSvc := inference.NewFake()
	// webhook service
	webhookSvc := webhook.NewFake(cfgSvc)

	svcs := pipeline.ServicesFactory{
		CfgSvc:       cfgSvc,
		DataSvc:      dataSvc,
		OrphanSvc:    orphanSvc,
		StorageSvc:   storageSvc,
		VmsSvc:       vmsSvc,
		InferenceSvc: inferenceSvc,
		WebhookSvc:   webhookSvc,
	}

	// Create mode processor result
	modeProcResult := make(chan error)
	defer close(modeProcResult)

	// Decide on streamers
	streamers := []pipeline.Streamer{
		// pipeline.SimpleDetector,
		// pipeline.MP4Recorder,
		pipeline.Yolo5Detector,
	}

	// Use the library simple alerter

	// Start the mode processor
	go func() {
		modeProcResult <- modeProc(canxCtx, svcs, streamers, pipeline.SimpleAlerter)
	}()

	// Wait for cancellation, mode proc, stats or error
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"agents pod context cancelled",
			)
			goto resume

		case err := <-modeProcResult:
			if err != nil {
				lgr.Logger.Info(
					"agents pod mode processor exited",
					slog.Any("error", xerrors.New(err.Error())),
				)
			}
			goto resume
		}
	}

	// Wait in a non-blocking way for `waitOnShutdown` for all the go routines to exit
	// This is needed because the go routines may need to report errors as they are existing
resume:
	// Cancel the context if not already cancelled
	if canxCtx.Err() == nil {
		// Force cancel the context
		canxFn()
	}

	lgr.Logger.Info(
		"agents pod is waiting for all go routines to exit",
	)

	// The only way to exit the main function is to wait for the shutdown
	// duration
	timer := time.NewTimer(waitOnShutdown)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"agents pod shutdown waiting period expired. Exiting now",
				slog.Duration("period", waitOnShutdown),
			)

			return

		case err := <-modeProcResult:
			if err != nil {
				lgr.Logger.Info(
					"agents pod mode processor exited",
					slog.Any("error", xerrors.New(err.Error())),
				)
			}
		}
	}
}
