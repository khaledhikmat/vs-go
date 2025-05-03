package pipeline

import (
	"context"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/inference"
	"github.com/khaledhikmat/vs-go/service/orphan"
	"github.com/khaledhikmat/vs-go/service/storage"
	"github.com/khaledhikmat/vs-go/service/vms"
	"github.com/khaledhikmat/vs-go/service/webhook"
	"gocv.io/x/gocv"
)

const (
	waitBeforeCancel = 3 * time.Second
)

type ServicesFactory struct {
	CfgSvc       config.IService
	DataSvc      data.IService
	OrphanSvc    orphan.IService
	StorageSvc   storage.IService
	VmsSvc       vms.IService
	InferenceSvc inference.IService
	WebhookSvc   webhook.IService
}

type FrameData struct {
	Mat       gocv.Mat
	Timestamp time.Time
}

type AlertData struct {
	Mat        gocv.Mat
	FrameURL   string
	ClipURL    string
	Camera     model.Camera
	Label      string
	Confidence float32
	Timestamp  time.Time
}

// Signature of streamer function
type Streamer func(canx context.Context, svcs ServicesFactory, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData

// Signature of alerter function
type Alerter func(canx context.Context, svcs ServicesFactory, errorStream chan interface{}, statsStream chan interface{}) chan AlertData
