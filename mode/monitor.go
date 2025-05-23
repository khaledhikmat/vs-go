package mode

import (
	"context"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/pipeline"
	"github.com/khaledhikmat/vs-go/service/lgr"
)

// The agents monitor is responsible for monitoring for orphaned cameras
// and publishing orphaned cameras so they can be picked up by the agents manager
func Monitor(canxCtx context.Context, svcs pipeline.ServicesFactory, _ []pipeline.Streamer, _ pipeline.Alerter) error {
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

		case <-time.After(time.Duration(time.Duration(svcs.CfgSvc.GetAgentsMonitorPeriodicTimeout()) * time.Second)):
			// Retrieve orphaned cameras
			cameras, err := svcs.DataSvc.RetrieveOrphanedCameras(10)
			if err != nil {
				errorStream <- model.GenError("agents_monitor",
					err,
					map[string]interface{}{},
					"error retrieving orphaned cameras")
				continue
			}

			// Publish orphaned cameras through the orphan service
			err = svcs.OrphanSvc.Publish(cameras)
			if err != nil {
				errorStream <- model.GenError("agents_monitor",
					err,
					map[string]interface{}{},
					"error publishing through orphan service")
				continue
			}

		case e := <-errorStream:
			procError(svcs.DataSvc, e)
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
	timer := time.NewTimer(time.Duration(svcs.CfgSvc.GetModeMaxShutdownTime()) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"agents monitor shutdown waiting period expired. Exiting now",
				slog.Duration("period", time.Duration(svcs.CfgSvc.GetModeMaxShutdownTime())*time.Second),
			)

			return nil

		case e := <-errorStream:
			procError(svcs.DataSvc, e)
		}
	}
}
