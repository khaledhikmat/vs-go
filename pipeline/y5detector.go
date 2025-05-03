package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"github.com/natefinch/lumberjack"
	"gocv.io/x/gocv"
)

// Global logger instance
var y5DetectionLogger = &lumberjack.Logger{
	Filename:   "detections.log",
	MaxSize:    10, // MB
	MaxBackups: 5,
	MaxAge:     7,    // days
	Compress:   true, // compress old logs
}

var y5AllowedClasses = map[string]bool{
	"person": true,
	"knife":  true,
	// Add more as needed
}

type y5Detection struct {
	Label      string          `json:"label"`
	Confidence float32         `json:"confidence"`
	Rect       image.Rectangle `json:"rect"`
}

func Yolo5Detector(canx context.Context, svcs ServicesFactory, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		//gocv.Set(gocv.LogLevelSilent)

		lgr.Logger.Info("yolo5 detector starting...",
			slog.String("camera", camera.Name),
			slog.String("model", svcs.CfgSvc.GetYolo5StreamerModelPath()),
			slog.String("openCV", gocv.Version()),
		)

		modelPath := svcs.CfgSvc.GetYolo5StreamerModelPath()
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			errorStream <- model.GenError("agent_yolo5_detector",
				fmt.Errorf("no yolo5 model exists"),
				map[string]interface{}{},
				"no yolo5 model exists")
			return
		}

		net := gocv.ReadNet(modelPath, "")
		if net.Empty() {
			errorStream <- model.GenError("agent_yolo5_detector",
				fmt.Errorf("error reading yolo5 model"),
				map[string]interface{}{},
				"error reading yolo5 model")
			return
		}
		defer net.Close()

		if err := net.SetPreferableBackend(gocv.NetBackendDefault); err != nil {
			errorStream <- model.GenError("agent_yolo5_detector", err, nil, "error setting backend")
			return
		}
		if err := net.SetPreferableTarget(gocv.NetTargetCPU); err != nil {
			errorStream <- model.GenError("agent_yolo5_detector", err, nil, "error setting target")
			return
		}

		labels := loadLabels(svcs.CfgSvc.GetYolo5CocoNamesPath())

		flush := func() {
			// TODO:
		}
		defer flush()

		lgr.Logger.Info("yolo5 detector initialized...",
			slog.String("model", modelPath),
		)

		proc := func(frame FrameData) {
			if frame.Mat.Empty() {
				fmt.Println("Skipping empty frame due to decode error")
				return
			}

			matClone := frame.Mat.Clone()
			defer matClone.Close()

			blob := gocv.BlobFromImage(matClone, 1.0/255.0, image.Pt(640, 640), gocv.NewScalar(0, 0, 0, 0), true, false)
			defer blob.Close()

			net.SetInput(blob, "")

			output := net.Forward("")
			defer output.Close()

			dims := output.Size()
			if len(dims) != 3 {
				fmt.Printf("Unexpected DNN output dims: %v\n", dims)
				return
			}

			reshaped := output.Reshape(1, dims[1])
			defer reshaped.Close()

			var allDetections []y5Detection

			for i := 0; i < reshaped.Rows(); i++ {
				row := reshaped.RowRange(i, i+1)
				data, okErr := row.DataPtrFloat32()
				row.Close()
				if okErr != nil || data == nil || len(data) < 5 {
					continue
				}

				dets := extractDetections(matClone, labels, data, svcs.CfgSvc.GetYolo5ConfidenceThreshold())
				allDetections = append(allDetections, dets...)
			}

			if len(allDetections) > 0 {
				logDetections(camera.Name, allDetections)
			}

			for _, det := range allDetections {
				fmt.Printf("ALERT: %s %.2f\n", det.Label, det.Confidence)
				select {
				case alertStream <- AlertData{
					Mat:        matClone.Clone(),
					Camera:     camera,
					Timestamp:  time.Now(),
					Label:      det.Label,
					Confidence: det.Confidence,
				}:
				default:
					lgr.Logger.Warn("alertStream full, dropping alert")
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

func extractDetections(frame gocv.Mat, labels []string, data []float32, confidenceThresh float32) []y5Detection {
	detections := []y5Detection{}

	if len(data) < 5 {
		fmt.Println("Skipping row: insufficient length", len(data))
		return detections
	}

	confidence := data[4]
	classScores := data[5:]
	if len(classScores) != len(labels) {
		fmt.Printf("Skipping row: classScores len=%d does not match labels len=%d\n", len(classScores), len(labels))
		return detections
	}

	classID := 0
	maxScore := float32(0.0)
	for j, score := range classScores {
		if score > maxScore {
			maxScore = score
			classID = j
		}
	}

	finalConf := confidence * maxScore

	//fmt.Printf("Row confidence: %f, class max score: %f (%s), finalConf: %f\n", confidence, maxScore, labels[classID], finalConf)

	if finalConf < confidenceThresh {
		return detections
	}

	if !y5AllowedClasses[strings.ToLower(labels[classID])] {
		return detections // Skip if class is not allowed
	}

	cx := data[0] * float32(frame.Cols())
	cy := data[1] * float32(frame.Rows())
	w := data[2] * float32(frame.Cols())
	h := data[3] * float32(frame.Rows())
	x := int(cx - w/2)
	y := int(cy - h/2)
	rect := image.Rect(x, y, x+int(w), y+int(h))

	detections = append(detections, y5Detection{
		Label:      labels[classID],
		Confidence: finalConf,
		Rect:       rect,
	})

	return detections
}

func logDetections(cameraName string, detections []y5Detection) {
	// Filter allowed classes
	filtered := []y5Detection{}
	for _, d := range detections {
		if y5AllowedClasses[strings.ToLower(d.Label)] {
			filtered = append(filtered, d)
		}
	}

	if len(filtered) == 0 {
		return // skip logging if none match
	}

	entry := map[string]interface{}{
		"time":       time.Now().Format(time.RFC3339),
		"camera":     cameraName,
		"detections": filtered,
	}

	jsonData, err := json.MarshalIndent(entry, "", "  ") // pretty-print
	if err != nil {
		fmt.Println("Error marshaling detections:", err)
		return
	}

	if _, err := y5DetectionLogger.Write(append(jsonData, '\n')); err != nil {
		fmt.Println("Error writing to log file:", err)
	}
}
