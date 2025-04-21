package mode

import (
	"context"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"github.com/khaledhikmat/vs-go/service/orphan"
)

// The agents monitor is responsible for monitoring for orphaned cameras
// and publishing orphaned cameras so they can be picked up by the agents manager
func Monitor(canxCtx context.Context, cfgSvc config.IService, dataSvc data.IService, orphanSvc orphan.IService) error {
	// Create an error stream
	errorStream := make(chan interface{})
	defer close(errorStream)

	// Wait for cancellation or timeout
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"agents monitor context cancelled",
			)
			goto resume

		case <-time.After(time.Duration(time.Duration(cfgSvc.GetAgentsMonitorPeriodicTimeout()) * time.Second)):
			// Retrieve orphaned cameras
			cameras, err := dataSvc.RetrieveOrphanedCameras(10)
			if err != nil {
				errorStream <- model.GenError("agents_monitor",
					err,
					map[string]interface{}{},
					"error retrieving orphaned cameras")
				continue
			}

			// Publish orphaned cameras through the orphan service
			err = orphanSvc.Publish(cameras)
			if err != nil {
				errorStream <- model.GenError("agents_monitor",
					err,
					map[string]interface{}{},
					"error publishing through orphan service")
				continue
			}

		case e := <-errorStream:
			procError(dataSvc, e)
		}
	}

	// Wait in a non-blocking way for `waitOnShutdown` seconds for all the go routines to exit
	// This is needed because the go routines may need to report errors as they are existing
resume:
	lgr.Logger.Info(
		"agents monitor is waiting for all go routines to exit",
	)

	// The only way to exit the main function is to wait for the shutdown
	// duration
	timer := time.NewTimer(time.Duration(cfgSvc.GetModeMaxShutdownTime()) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"agents monitor shutdown waiting period expired. Exiting now",
				slog.Duration("period", time.Duration(cfgSvc.GetModeMaxShutdownTime())*time.Second),
			)

			return nil

		case e := <-errorStream:
			procError(dataSvc, e)
		}
	}
}
