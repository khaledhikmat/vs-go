package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
)

func SimpleDetector(canx context.Context, svcs ServicesFactory, camera model.Camera, _ chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		lgr.Logger.Info(
			"simple detector initialized...",
			slog.String("camera", camera.Name),
		)

		flush := func() {
			// TODO:
		}
		defer flush()

		proc := func(frame FrameData, frames, worker int) {
			defer frame.Mat.Close()

			lgr.Logger.Debug(
				"simple detector processing frame",
				slog.Int("frames", frames),
				slog.Int("worker", worker),
			)

			// TODO: Add your detection logic here
			time.Sleep(10 * time.Millisecond) // Simulate processing time

			// For testing purposes, we will send an alert every 1000 frames
			if (worker == 0 && frames == 1000) ||
				(worker == 1 && frames == 2000) ||
				(worker == 2 && frames == 3000) {
				// Send alert to the alert stream
				alertStream <- AlertData{
					Mat:        frame.Mat.Clone(),
					FrameURL:   "",
					ClipURL:    "",
					Camera:     camera,
					Timestamp:  time.Now(),
					Label:      "simple",
					Confidence: 100.0,
				}
			}
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
					if fps == 0 {
						fps = 1
					}

					// Calculate average processing time
					var AvgProcTime float64
					if frames > 0 {
						AvgProcTime = totalInferenceTime.Seconds() / float64(frames)
					}

					statsStream <- model.StreamerStats{
						Name:        "simpleDetector",
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
							"simple detector worker context cancelled",
							slog.Int("worker", worker),
						)
						return
					default:
						// Process frame
						startInference := time.Now()
						proc(f, frames, worker)
						frames++
						totalInferenceTime += time.Since(startInference) // Accumulate processing time
					}
				}
			}(worker, in)
		}

		// Wait until cancelled
		<-canx.Done()
		// Give some time to the framer to recognize the context is cancelled
		time.Sleep(waitBeforeCancel)
		lgr.Logger.Info(
			"simple detector context cancelled",
		)
	}()

	return in
}
