package pipeline

import (
	"context"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"gocv.io/x/gocv"
)

type FrameData struct {
	Mat       gocv.Mat
	Timestamp time.Time
}

type AlertData struct {
	Mat        gocv.Mat
	Camera     model.Camera
	Label      string
	Confidence float32
	Timestamp  time.Time
}

// Signature of streamer function
type Streamer func(canx context.Context, cfgsvc config.IService, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData

// Signature of alerter function
type Alerter func(canx context.Context, cfgsvc config.IService, errorStream chan interface{}, statsStream chan interface{}) chan AlertData
