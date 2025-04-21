package orphan

import (
	"context"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"golang.org/x/xerrors"
)

type timedService struct {
	CanxCtx       context.Context
	SubsCtx       context.Context
	SubsCancel    context.CancelFunc
	CameraChannel chan []model.Camera
	CfgSvc        config.IService
	DataSvc       data.IService
	Cameras       []model.Camera
}

// This implementation provides timed orphan service where the service delivers on its
// subscribed channel one orphaned camera every 5 seconds.
func NewTimed(canxCtx context.Context, cfgSvc config.IService, dataSvc data.IService) IService {
	cameras, err := dataSvc.RetrieveCameras()
	if err != nil {
		lgr.Logger.Error(
			"error unmarshalling json",
			slog.Any("error", xerrors.New(err.Error())),
		)
		panic("error unmarshalling json")
	}

	return &timedService{
		CfgSvc:  cfgSvc,
		DataSvc: dataSvc,
		CanxCtx: canxCtx,
		Cameras: cameras,
	}
}

func (svc *timedService) Publish(_ []model.Camera) error {
	// This cannot be implemented in this service
	return nil
}

func (svc *timedService) Subscribe() (<-chan []model.Camera, error) {
	if svc.SubsCtx != nil {
		lgr.Logger.Error(
			"orphan timed service. Alreday subscribed to cameras. Unsubscribe first",
			slog.Any("Subs Context", svc.SubsCtx),
		)
		return nil, xerrors.New("orphan timed service. child context is not nil. Unsubscribe first")
	}

	// Create a channel to send orphaned cameras that need agents
	// This is created the first time we subscribe
	// Regardless of how many times we subscribe/unsubscribe, we will always
	// have only one channel to send the cameras to the agent manager
	if svc.CameraChannel == nil {
		svc.CameraChannel = make(chan []model.Camera)
	}

	// Create a child context for the subscription
	// This context will be used to cancel the subscription
	subsContext, subsCancel := context.WithCancel(svc.CanxCtx)
	svc.SubsCtx = subsContext
	svc.SubsCancel = subsCancel

	// Start a goroutine to simulate subscribing to camera data
	go func() {
		defer svc.cleanup()

		cameraIndex := 0

		// Wait for cancellation or timeout to simulate agent manager
		for {
			select {
			case <-svc.CanxCtx.Done():
				lgr.Logger.Info(
					"orphan timed service context cancelled",
				)
				return
			case <-svc.SubsCtx.Done():
				lgr.Logger.Info(
					"orphan timed service context cancelled",
				)
				return
			case <-time.After(time.Duration(5 * time.Second)):
				if cameraIndex >= len(svc.Cameras) {
					cameraIndex = 0
				}

				svc.CameraChannel <- []model.Camera{svc.Cameras[cameraIndex]}
				cameraIndex++
			}
		}
	}()

	return svc.CameraChannel, nil
}

func (svc *timedService) Unsubscribe() error {
	if svc.SubsCtx == nil {
		return xerrors.New("No subscribed yet. Subscribe first")
	}

	svc.cleanup()
	return nil
}

func (svc *timedService) Finalize() {
	if svc.CameraChannel != nil {
		close(svc.CameraChannel)
		svc.CameraChannel = nil
	}
}

func (svc *timedService) cleanup() {
	if svc.SubsCancel != nil {
		svc.SubsCancel()
		svc.SubsCtx = nil
		svc.SubsCancel = nil
	}
}
