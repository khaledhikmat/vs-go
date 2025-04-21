package pipeline

import (
	"context"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

func framer(canxCtx context.Context, cfgSvc config.IService, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, streamChannels []chan FrameData) {
	if camera.FramerType == "random" {
		go randomFramer(canxCtx, cfgSvc, camera, errorStream, statsStream, streamChannels)
		return
	}

	go rtspFramer(canxCtx, cfgSvc, camera, errorStream, statsStream, streamChannels)
}

func rtspFramer(canxCtx context.Context, _ config.IService, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, streamChannels []chan FrameData) {
	webcam, err := gocv.OpenVideoCapture(camera.RtspURL)
	if err != nil {
		errorStream <- model.GenError("agent_rtsp_framer",
			err,
			map[string]interface{}{},
			"error opening RTSP stream")
		return
	}
	defer webcam.Close()

	var startTime = time.Now().Unix()
	var endTime = time.Now().Unix()
	var frames = 0
	var errors = 0

	defer func() {
		endTime = time.Now().Unix()
		uptime := endTime - startTime
		fps := int(float64(frames) / float64(uptime))
		statsStream <- model.FramerStats{
			Name:   "rtspFramer",
			Camera: camera.Name,
			Frames: frames,
			Errors: errors,
			Uptime: uptime,
			FPS:    fps,
		}
	}()

	// Capture frames, route captured frames to streamers and monitor cancellations
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"rtspFramer context cancelled",
			)
			return

		default:
			img := gocv.NewMat()
			if ok := webcam.Read(&img); !ok || img.Empty() {
				errors++
				img.Close()
				continue
			}

			frames++
			for _, streamChan := range streamChannels {
				// WARNING: We need an extra check to make sure we don't send on c closed channel
				select {
				case streamChan <- FrameData{Mat: img.Clone(), Timestamp: time.Now()}:
					// Successfully sent to the channel
				case <-canxCtx.Done():
					// Context canceled, stop sending
					lgr.Logger.Info("rtspFramer context cancelled while sending!!")
				}
			}

			img.Close()
		}
	}
}

func randomFramer(canxCtx context.Context, _ config.IService, camera model.Camera, _ chan interface{}, statsStream chan interface{}, streamChannels []chan FrameData) {
	var startTime = time.Now().Unix()
	var endTime = time.Now().Unix()
	var frames = 0
	var errors = 0

	defer func() {
		endTime = time.Now().Unix()
		uptime := endTime - startTime
		fps := int(float64(frames) / float64(uptime))
		statsStream <- model.FramerStats{
			Name:   "randomFramer",
			Camera: camera.Name,
			Frames: frames,
			Errors: errors,
			Uptime: uptime,
			FPS:    fps,
		}
	}()

	// Capture frames, route captured frames to streamers and monitor cancellations
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"randomFramer context cancelled",
			)
			return
		default:
			frames++
			// Generate a random frame
			img := gocv.NewMatWithSize(480, 640, gocv.MatTypeCV8UC3) // Create a 480x640 image with 3 channels (BGR)
			// Route the frame to multiple streamers
			for _, streamChan := range streamChannels {
				// WARNING: We need an extra check to make sure we don't send on c closed channel
				select {
				case streamChan <- FrameData{Mat: img.Clone(), Timestamp: time.Now()}:
					// Successfully sent to the channel
				case <-canxCtx.Done():
					// Context canceled, stop sending
					lgr.Logger.Info("randomFramer context cancelled while sending!!")
				}
			}

			img.Close()
		}
	}
}
