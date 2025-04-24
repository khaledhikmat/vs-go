package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log/slog"
	"strings"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

func SimpleDetector(canx context.Context, cfgSvc config.IService, camera model.Camera, _ chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

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

			// TODO: Generate an alert every n frames
			if frames > 0 && frames%5000 == 0 {
				// Send alert to the alert stream
				alertStream <- AlertData{
					Mat:        frame.Mat.Clone(),
					Camera:     camera,
					Timestamp:  time.Now(),
					Label:      "simple",
					Confidence: 100.0,
				}
			}
		}

		// Launch worker processes that compete on emptying/procesing frames
		for i := 0; i < cfgSvc.GetStreamerMaxWorkers(); i++ {
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
		lgr.Logger.Info(
			"simple detector context cancelled",
		)
	}()

	return in
}

var (
	modelPath     = "yolov5s.onnx"
	labelsPath    = "coco.names"
	confThreshold = float32(0.5)
)

func Yolo5Detector(canx context.Context, cfgSvc config.IService, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		net := gocv.ReadNet(modelPath, "")
		err := net.SetPreferableBackend(gocv.NetBackendDefault)
		if err != nil {
			errorStream <- model.GenError("agent_yolo5_detector",
				err,
				map[string]interface{}{},
				"error setting backend")
			return
		}
		err = net.SetPreferableTarget(gocv.NetTargetCPU)
		if err != nil {
			errorStream <- model.GenError("agent_yolo5_detector",
				err,
				map[string]interface{}{},
				"error setting target")
			return
		}

		labels := loadLabels(labelsPath)

		flush := func() {
			// TODO:
		}
		defer flush()

		proc := func(frame FrameData) {
			defer frame.Mat.Close()

			blob := gocv.BlobFromImage(frame.Mat, 1.0/255.0, image.Pt(640, 640), gocv.NewScalar(0, 0, 0, 0), true, false)
			net.SetInput(blob, "images") // TODO: may not work!!
			output := net.Forward("")
			blob.Close()

			dims := output.Size()
			for i := 0; i < dims[1]; i++ {
				row := output.RowRange(i, i+1)
				data, _ := row.DataPtrFloat32()
				confidence := data[4]
				if confidence < confThreshold {
					continue
				}

				classScores := data[5:]
				classID := 0
				maxScore := float32(0.0)
				for j, score := range classScores {
					if score > maxScore {
						maxScore = score
						classID = j
					}
				}

				if maxScore > confThreshold {
					cx, cy, w, h := data[0]*float32(frame.Mat.Cols()), data[1]*float32(frame.Mat.Rows()), data[2]*float32(frame.Mat.Cols()), data[3]*float32(frame.Mat.Rows())
					x := int(cx - w/2)
					y := int(cy - h/2)
					rect := image.Rect(x, y, x+int(w), y+int(h))

					label := labels[classID]
					gocv.Rectangle(&frame.Mat, rect, color.RGBA{0, 255, 0, 0}, 2)
					gocv.PutText(&frame.Mat, fmt.Sprintf("%s %.2f", label, maxScore), image.Pt(x, y-5),
						gocv.FontHersheySimplex, 0.6, color.RGBA{0, 255, 0, 0}, 2)

					// Send alert to the alert stream
					alertStream <- AlertData{
						Mat:        frame.Mat.Clone(),
						Camera:     camera,
						Timestamp:  time.Now(),
						Label:      label,
						Confidence: maxScore,
					}
				}
			}
		}

		// Launch worker processes that compete on emptying/procesing frames
		for i := 0; i < cfgSvc.GetStreamerMaxWorkers(); i++ {
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
						Name:        "yolo5Detector",
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
							"yolo5Detector detector worker context cancelled",
							slog.Int("worker", worker),
						)
						return
					default:
						// Process frame
						startInference := time.Now()
						proc(f)
						frames++
						totalInferenceTime += time.Since(startInference) // Accumulate processing time
					}
				}
			}(worker, in)
		}

		// Wait until cancelled
		<-canx.Done()
		lgr.Logger.Info(
			"yolo5Detector detector context cancelled",
		)
	}()

	return in
}

func loadLabels(path string) []string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func WebrtcBroadcaster(canx context.Context, cfgSvc config.IService, camera model.Camera, _ chan interface{}, statsStream chan interface{}, _ chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		flush := func() {
			// TODO:
		}
		defer flush()

		proc := func(frame FrameData) {
			defer frame.Mat.Close()

			// Add logic
		}

		// Launch worker processes that compete on emptying/procesing frames
		for i := 0; i < cfgSvc.GetStreamerMaxWorkers(); i++ {
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

func MP4Recorder(canx context.Context, cfgSvc config.IService, camera model.Camera, _ chan interface{}, statsStream chan interface{}, _ chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		var buffer []FrameData
		var recordingTime = time.Now()

		flush := func(buffer []FrameData) {
			if len(buffer) > 0 {
				saveFramesAsMP4(cfgSvc, camera, buffer)
				for _, f := range buffer {
					f.Mat.Close()
				}
			}
		}

		proc := func(frame FrameData) bool {
			buffer = append(buffer, frame)

			if time.Since(recordingTime) >= time.Duration(cfgSvc.GetRecorderStreamerClipDuration())*time.Second {
				// WARNING: Not sure if we need to close images in closed buffer
				// Clone the buffer
				clonedBuffer := make([]FrameData, len(buffer))
				copy(clonedBuffer, buffer)
				// Close all images of the original buffer...not sure
				for _, f := range buffer {
					f.Mat.Close()
				}

				// Flush the cloned buffer asynchronously
				// so we don't block the main loop
				go flush(clonedBuffer)
				return true
			}

			return false
		}

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
				Name:        "mp4Recorder",
				Worker:      -1,
				Camera:      camera.Name,
				Frames:      frames,
				Errors:      errors,
				Uptime:      uptime,
				FPS:         fps,
				AvgProcTime: AvgProcTime,
			}
		}()

		defer flush(buffer)

		// WARNING: Can't do workers because we need the frames to be in order
		for f := range in {
			select {
			case <-canx.Done():
				lgr.Logger.Info(
					"recorder context cancelled",
				)
				return
			default:
				// Process frame
				startInference := time.Now()
				if proc(f) {
					// Reset buffer if we flushed
					buffer = nil
					recordingTime = time.Now()
				}
				frames++
				totalInferenceTime += time.Since(startInference) // Accumulate processing time
			}
		}
	}()

	return in
}

func saveFramesAsMP4(cfgSvc config.IService, camera model.Camera, frames []FrameData) {
	if len(frames) == 0 {
		return
	}

	filename := fmt.Sprintf("%s/%s_recording_%d.mp4", cfgSvc.GetRecordingsFolder(), camera.Name, time.Now().Unix())
	writer, err := gocv.VideoWriterFile(filename, "avc1", 30, frames[0].Mat.Cols(), frames[0].Mat.Rows(), true)
	if err != nil {
		lgr.Logger.Error(
			"error creating video writer",
			slog.Any("error", err),
		)
		return
	}
	defer writer.Close()

	for _, f := range frames {
		writer.Write(f.Mat)
	}
}
