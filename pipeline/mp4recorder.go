package pipeline

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"os"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

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

		lgr.Logger.Info(
			"mp4 recorder initialized...",
			slog.String("camera", camera.Name),
		)

		flush := func(clonedBuffer []FrameData) {
			defer func() {
				for _, f := range clonedBuffer {
					f.Mat.Close()
				}
				if r := recover(); r != nil {
					lgr.Logger.Error("flush panic recovered:", r)
				}
			}()

			if len(clonedBuffer) == 0 {
				return
			}

			fn, err := saveFramesAsMP4(svcs, camera, clonedBuffer)
			if err != nil {
				errorStream <- model.GenError("agent_mp4_recorder",
					err,
					map[string]interface{}{},
					"error saving frames as mp4")
				return
			}

			defer func() {
				if fn == "" {
					return
				}

				err := os.Remove(fn)
				if err != nil {
					errorStream <- model.GenError("agent_mp4_recorder",
						err,
						map[string]interface{}{},
						"error deleting the local clip %s",
						fn)
				}
			}()

			if fn != "" {
				clipURL, err := svcs.StorageSvc.StoreFile(fn)
				if err != nil {
					errorStream <- model.GenError("agent_mp4_recorder",
						err,
						map[string]interface{}{},
						"error storing a clip %s",
						fn)
					return
				}

				result, err := svcs.InferenceSvc.Invoke("", clipURL)
				if err != nil {
					errorStream <- model.GenError("agent_mp4_recorder",
						err,
						map[string]interface{}{},
						"error invoking a clip inference %s",
						fn)
					return
				}

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

		proc := func(frame FrameData) bool {
			buffer = append(buffer, frame)

			if time.Since(recordingTime) >= time.Duration(svcs.CfgSvc.GetStreamerParameters(config.MP4RecorderName).ClipDuration)*time.Second {
				clonedBuffer := make([]FrameData, len(buffer))
				for i, f := range buffer {
					clonedBuffer[i] = FrameData{
						Mat:       f.Mat.Clone(),
						Timestamp: f.Timestamp,
					}
				}

				// Close original frames immediately (not deferred)
				for _, f := range buffer {
					f.Mat.Close()
				}

				// Reset buffer slice and capacity
				buffer = make([]FrameData, 0, len(buffer))

				// Launch flush as a goroutine
				go flush(clonedBuffer)
				return true
			}
			return false
		}

		frames := 0
		beginTime := time.Now().Unix()
		endTime := time.Now().Unix()
		errors := 0
		var totalInferenceTime time.Duration

		defer func() {
			endTime = time.Now().Unix()
			uptime := endTime - beginTime
			fps := int(float64(frames) / float64(uptime))

			var avgProcTime float64
			if frames > 0 {
				avgProcTime = totalInferenceTime.Seconds() / float64(frames)
			}

			statsStream <- model.StreamerStats{
				Name:        "mp4Recorder",
				Worker:      -1,
				Camera:      camera.Name,
				Frames:      frames,
				Errors:      errors,
				Uptime:      uptime,
				FPS:         fps,
				AvgProcTime: avgProcTime,
			}
		}()

		defer func() {
			// Final flush on shutdown
			if len(buffer) > 0 {
				clonedBuffer := make([]FrameData, len(buffer))
				for i, f := range buffer {
					clonedBuffer[i] = FrameData{
						Mat:       f.Mat.Clone(),
						Timestamp: f.Timestamp,
					}
					f.Mat.Close()
				}
				go flush(clonedBuffer)
			}
		}()

		for f := range in {
			select {
			case <-canx.Done():
				lgr.Logger.Info("recorder context cancelled")
				time.Sleep(waitBeforeCancel)
				return
			default:
				startInference := time.Now()
				if proc(f) {
					recordingTime = time.Now()
				}
				frames++
				totalInferenceTime += time.Since(startInference)
			}
		}
	}()

	return in
}

func saveFramesAsMP4(svcs ServicesFactory, camera model.Camera, frames []FrameData) (string, error) {
	lgr.Logger.Info(
		"mp4 recorder saveFramesAsMP4",
		slog.String("camera", camera.Name),
		slog.Int("frames", len(frames)),
	)

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
	lgr.Logger.Info(
		"mp4 recorder saveFramesAsMP4 - phase2",
		slog.String("camera", camera.Name),
		slog.Int("frames", len(frames)),
		slog.String("filename", filename),
	)

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
