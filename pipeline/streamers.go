package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

const (
	waitBeforeCancel = 3 * time.Second
)

func SimpleDetector(canx context.Context, svcs ServicesFactory, camera model.Camera, _ chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
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

func Yolo5Detector(canx context.Context, svcs ServicesFactory, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		net := gocv.ReadNet(svcs.CfgSvc.GetYolo5StreamerModelPath(), "")
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

		labels := loadLabels(svcs.CfgSvc.GetYolo5CocoNamesPath())

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
				if confidence < svcs.CfgSvc.GetYolo5ConfidenceThreshold() {
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

				if maxScore > svcs.CfgSvc.GetYolo5ConfidenceThreshold() {
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
		// Give some time to the framer to recognize the context is cancelled
		time.Sleep(waitBeforeCancel)
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

// WARNING:
// GoCV is not an optimal choice for MP4 Recording.
// This is because GoCV produces uncompressed frames, which might generate large/huge MP4 files.
// GoCV is optimized for frame processing and inference.
// RTSP Low-level library is used for WebRTC broadcasting.
func MP4Recorder(canx context.Context, svcs ServicesFactory, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		var buffer []FrameData
		var recordingTime = time.Now()

		flush := func(clonedBuffer []FrameData) {
			if len(buffer) > 0 {
				fn, err := saveFramesAsMP4(svcs, camera, clonedBuffer)
				if err != nil {
					errorStream <- model.GenError("agent_mp4_recorder",
						err,
						map[string]interface{}{},
						"error saving frames as mp4")
					return
				}

				// close all images of the cloned buffer
				defer func() {
					for _, f := range clonedBuffer {
						f.Mat.Close()
					}
				}()

				// Delete the file locally
				defer func() {
					err := os.Remove(fn)
					if err != nil {
						errorStream <- model.GenError("agent_mp4_recorder",
							err,
							map[string]interface{}{},
							"error deleting the local clip %s",
							fn)
					}
				}()

				// This is happening on a different thread
				// Process the clip
				// Not all saves return a filename
				if fn != "" {
					var err error
					// Store the alerted image possibly to a cloud storage
					clipURL, err := svcs.StorageSvc.StoreFile(fn)
					if err != nil {
						errorStream <- model.GenError("agent_mp4_recorder",
							err,
							map[string]interface{}{},
							"error storing a clip %s",
							fn)
						return
					}

					// Invoke the model layer using this S3 file
					result, err := svcs.InferenceSvc.Invoke("", clipURL)
					if err != nil {
						errorStream <- model.GenError("agent_mp4_recorder",
							err,
							map[string]interface{}{},
							"error invoking a clip inference %s",
							fn)
						return
					}

					// Generate an alert if the inference service returns an alert
					if result.AlertImageURL != "" {
						alertStream <- AlertData{
							FrameURL:   result.AlertImageURL,
							ClipURL:    clipURL,
							Camera:     camera,
							Timestamp:  time.Now(),
							Label:      "simple",
							Confidence: 100.0,
						}
					}
				}
			}
		}

		proc := func(frame FrameData) bool {
			buffer = append(buffer, frame)

			if time.Since(recordingTime) >= time.Duration(svcs.CfgSvc.GetRecorderStreamerClipDuration())*time.Second {
				// WARNING: Not sure if we need to close images in closed buffer
				// Clone the buffer
				clonedBuffer := make([]FrameData, len(buffer))
				//copy(clonedBuffer, buffer)
				// Perform a deep copy of the buffer
				for i, f := range buffer {
					clonedBuffer[i] = FrameData{
						Mat: f.Mat.Clone(),
					}
				}

				defer func() {
					// Close all images of the original buffer...not sure
					for _, f := range buffer {
						f.Mat.Close()
					}
				}()

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
				// Give some time to the framer to recognize the context is cancelled
				time.Sleep(waitBeforeCancel)
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

func saveFramesAsMP4(svcs ServicesFactory, camera model.Camera, frames []FrameData) (string, error) {
	if len(frames) == 0 {
		return "", fmt.Errorf("no frames to save")
	}

	if frames[0].Mat.Empty() {
		lgr.Logger.Error(
			"frames[0].Mat is empty or invalid",
			slog.Int("frame_index", 0),
			slog.Int("cols", frames[0].Mat.Cols()),
			slog.Int("rows", frames[0].Mat.Rows()),
		)
		return "", fmt.Errorf("invalid Mat in frames[0]")
	}

	if frames[0].Mat.Cols() <= 0 || frames[0].Mat.Rows() <= 0 {
		lgr.Logger.Error(
			"invalid frame dimensions",
			slog.Int("cols", frames[0].Mat.Cols()),
			slog.Int("rows", frames[0].Mat.Rows()),
		)
		return "", fmt.Errorf("invalid frame dimensions: cols=%d, rows=%d", frames[0].Mat.Cols(), frames[0].Mat.Rows())
	}

	filename := fmt.Sprintf("%s/%s_recording_%d.mp4", svcs.CfgSvc.GetRecordingsFolder(), camera.Name, time.Now().Unix())
	writer, err := gocv.VideoWriterFile(filename, "avc1", 30, frames[0].Mat.Cols(), frames[0].Mat.Rows(), true)
	if err != nil {
		lgr.Logger.Error(
			"error creating video writer",
			slog.Any("error", err),
		)
		return "", err
	}
	defer writer.Close()

	for _, f := range frames {
		// Check if the frame dimensions match the video dimensions
		if f.Mat.Cols() != frames[0].Mat.Cols() || f.Mat.Rows() != frames[0].Mat.Rows() {
			lgr.Logger.Warn(
				"frame dimensions do not match video dimensions, resizing frame",
				slog.Int("frame_cols", f.Mat.Cols()),
				slog.Int("frame_rows", f.Mat.Rows()),
				slog.Int("video_cols", frames[0].Mat.Cols()),
				slog.Int("video_rows", frames[0].Mat.Rows()),
			)

			// Resize the frame to match the video dimensions
			resized := gocv.NewMat()
			defer resized.Close()
			err := gocv.Resize(f.Mat, &resized, image.Pt(frames[0].Mat.Cols(), frames[0].Mat.Rows()), 0, 0, gocv.InterpolationLinear)
			if err != nil {
				lgr.Logger.Error(
					"error creating video writer",
					slog.Any("error", err),
				)
				return "", err
			}

			// Write the resized frame to the video
			err = writer.Write(resized)
			if err != nil {
				lgr.Logger.Error(
					"error creating video writer",
					slog.Any("error", err),
				)
				return "", err
			}
		} else {
			// Write the frame as is
			err := writer.Write(f.Mat)
			if err != nil {
				lgr.Logger.Error(
					"error creating video writer",
					slog.Any("error", err),
				)
				return "", err
			}
		}
	}

	return filename, nil
}
