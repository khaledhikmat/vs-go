package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
)

// WARNING:
// GoCV is not an optimal choice for WebRTC broadcasting.
// This is because GoCV produces uncompressed frames, which are not suitable for WebRTC.
// GoCV is optimized for frame processing and inference.
// RTSP Low-level library is used for WebRTC broadcasting.
// The frames need to be compressed before being sent over the network.
// The frames need to be converted to a format that is compatible with WebRTC, which can be a bottleneck in the streaming process.
func WebrtcBroadcaster(canx context.Context, svcs ServicesFactory, camera model.Camera, _ chan interface{}, statsStream chan interface{}, _ chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		lgr.Logger.Info(
			"webrtc broadcaster initialized...",
			slog.String("camera", camera.Name),
		)

		flush := func() {
			// TODO:
		}
		defer flush()

		proc := func(frame FrameData) {
			defer frame.Mat.Close()

			// Add logic
		}

		// Launch worker processes that compete on emptying/procesing frames
		for i := 0; i < svcs.CfgSvc.GetStreamerMaxWorkers(); i++ {
			worker := i // Capture the loop variable
			go func(worker int, in chan FrameData) {
				frames := 0
				beginTime := time.Now().Unix()
				endTime := time.Now().Unix()
				errors := 0

				var totalInferenceTime time.Duration // Track total processing time

				defer func() {
					endTime = time.Now().Unix()
					uptime := endTime - beginTime
					fps := int(float64(frames) / float64(uptime))

					// Calculate average processing time
					var AvgProcTime float64
					if frames > 0 {
						AvgProcTime = totalInferenceTime.Seconds() / float64(frames)
					}

					statsStream <- model.StreamerStats{
						Name:        "webrtcBroadcaster",
						Worker:      worker,
						Camera:      camera.Name,
						Frames:      frames,
						Errors:      errors,
						Uptime:      uptime,
						FPS:         fps,
						AvgProcTime: AvgProcTime,
					}
				}()

				for f := range in {
					select {
					case <-canx.Done():
						lgr.Logger.Info(
							"webrtcBroadcaster worker context cancelled",
							slog.Int("worker", worker),
						)
						return
					default:
						// Process frame
						startInference := time.Now()
						proc(f)
						totalInferenceTime += time.Since(startInference) // Accumulate processing time
					}
				}
			}(worker, in)
		}

		// Wait until cancelled
		<-canx.Done()
		lgr.Logger.Info(
			"webrtcBroadcaster context cancelled",
		)
	}()

	return in
}
