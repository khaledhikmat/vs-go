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
	"sync"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
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

var y5RowLogger = &lumberjack.Logger{
	Filename:   "rows.log",
	MaxSize:    1000, // MB
	MaxBackups: 5,
	MaxAge:     7,    // days
	Compress:   true, // compress old logs
}

var y5AllowedClasses = map[string]bool{
	"person": true,
	// Add more as needed
}

type y5Detection struct {
	Label            string          `json:"label"`
	ObjectConfidence float32         `json:"objectConfidence"`
	ClassConfidence  float32         `json:"classConfidence"`
	Confidence       float32         `json:"confidence"`
	Rect             image.Rectangle `json:"rect"`
}

func Yolo5Detector(canx context.Context, svcs ServicesFactory, camera model.Camera, errorStream chan interface{}, statsStream chan interface{}, alertStream chan AlertData) chan FrameData {
	in := make(chan FrameData, 100)

	go func() {
		defer close(in)

		lgr.Logger.Info("yolo5 detector starting...",
			slog.String("camera", camera.Name),
			slog.String("model", svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).ModelPath),
			slog.String("openCV", gocv.Version()),
		)

		modelPath := svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).ModelPath
		if _, err := os.Stat(modelPath); os.IsNotExist(err) {
			errorStream <- model.GenError("agent_yolo5_detector",
				fmt.Errorf("no yolo5 model exists"),
				map[string]interface{}{},
				"no yolo5 model exists")
			return
		}

		labels := loadLabels(svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).CocoNamesPath)

		var lastAlertTime = make(map[string]time.Time)
		var alertMutex = sync.Mutex{}
		var cooldown = time.Duration(svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).CoolDownPeriod) * time.Second

		proc := func(frame FrameData, net *gocv.Net) {
			defer frame.Mat.Close()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Recovered from panic: %v\n", r)
				}
			}()

			if frame.Mat.Empty() {
				fmt.Println("Skipping empty frame due to decode error")
				return
			}

			blob := gocv.BlobFromImage(frame.Mat, 1.0/255.0, image.Pt(640, 640), gocv.NewScalar(0, 0, 0, 0), true, false)
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
			if reshaped.Empty() || reshaped.Rows() == 0 || reshaped.Cols() < 5 {
				fmt.Println("Reshape failed or invalid dimensions")
				reshaped.Close()
				return
			}
			defer reshaped.Close()

			var allDetections []y5Detection
			for i := 0; i < reshaped.Rows(); i++ {
				row := reshaped.RowRange(i, i+1)
				data, okErr := row.DataPtrFloat32()
				row.Close()

				if okErr != nil || data == nil || len(data) < 5 {
					continue
				}

				if data[4] < svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).ObjectConfidenceThreshold {
					continue
				}

				dets := extractDetections(i, frame.Mat, labels, data,
					svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).ConfidenceThreshold,
					svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).ObjectConfidenceThreshold,
					svcs.CfgSvc.GetStreamerParameters(config.Yolo5DetectorName).Logging)
				allDetections = append(allDetections, dets...)
			}

			if len(allDetections) == 0 {
				return
			}

			// Find the best detection
			maxConfidence := float32(0.0)
			bestDetection := y5Detection{}
			for _, det := range allDetections {
				if det.Confidence > maxConfidence {
					maxConfidence = det.Confidence
					bestDetection = det
				}
			}

			shouldAlert := false
			alertMutex.Lock()
			lastTime, exists := lastAlertTime[bestDetection.Label]
			if !exists || time.Since(lastTime) > cooldown {
				shouldAlert = true
				lastAlertTime[bestDetection.Label] = time.Now()
			}
			alertMutex.Unlock()

			if !shouldAlert {
				return
			}

			select {
			case alertStream <- AlertData{
				Mat:        frame.Mat.Clone(),
				Camera:     camera,
				Timestamp:  time.Now(),
				Label:      bestDetection.Label,
				Confidence: bestDetection.ObjectConfidence * bestDetection.ClassConfidence,
			}:
			default:
				lgr.Logger.Warn("alertStream full, dropping alert")
			}
		}

		for i := 0; i < svcs.CfgSvc.GetStreamerMaxWorkers(); i++ {
			worker := i
			go func(worker int, in chan FrameData) {
				// WARNING: net is not thread-safe!!!
				// So it must be created in each worker
				net := gocv.ReadNet(modelPath, "")
				if net.Empty() {
					errorStream <- model.GenError("agent_yolo5_detector",
						fmt.Errorf("worker %d: error reading yolo5 model", worker),
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

				frames := 0
				beginTime := time.Now().Unix()
				endTime := time.Now().Unix()
				errors := 0
				var totalInferenceTime time.Duration

				defer func() {
					endTime = time.Now().Unix()
					uptime := endTime - beginTime
					fps := int(float64(frames) / float64(uptime))
					if fps == 0 {
						fps = 1
					}
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
						startInference := time.Now()
						proc(f, &net)
						frames++
						totalInferenceTime += time.Since(startInference)
					}
				}
			}(worker, in)
		}

		<-canx.Done()
		time.Sleep(waitBeforeCancel)
		lgr.Logger.Info("yolo5Detector detector context cancelled")
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

func extractDetections(idx int, frame gocv.Mat, labels []string, data []float32, confidenceThresh float32, objectConfidenceThresh float32, logging bool) []y5Detection {
	detections := []y5Detection{}

	if len(data) < 5 {
		fmt.Println("Skipping row: insufficient length", len(data))
		return detections
	}

	objectConfidence := data[4] // objectness
	classScores := data[5:]

	if len(classScores) != len(labels) {
		fmt.Printf("Skipping row: classScores len=%d does not match labels len=%d\n", len(classScores), len(labels))
		return detections
	}

	classID := -1
	classConfidence := float32(0.0)
	for j, score := range classScores {
		label := strings.ToLower(labels[j])
		if !y5AllowedClasses[label] {
			continue
		}
		if score > classConfidence {
			classConfidence = score
			classID = j
		}
	}

	finalConf := objectConfidence * classConfidence

	// Ignore if the class is not important to us or the object and class confidences are low
	if classID == -1 ||
		objectConfidence < objectConfidenceThresh ||
		finalConf < confidenceThresh {
		return detections
	}

	if logging {
		logRows("camera", "post", fmt.Sprintf("Row %d confidence: %f, class max score: %f (%s), finalConf: %f, class ID: %d\n", idx, objectConfidence, classConfidence, labels[classID], finalConf, classID))
	}

	if !y5AllowedClasses[strings.ToLower(labels[classID])] {
		return detections
	}

	cx := data[0] * float32(frame.Cols())
	cy := data[1] * float32(frame.Rows())
	w := data[2] * float32(frame.Cols())
	h := data[3] * float32(frame.Rows())
	x := int(cx - w/2)
	y := int(cy - h/2)
	rect := image.Rect(x, y, x+int(w), y+int(h))

	detections = append(detections, y5Detection{
		Label:            labels[classID],
		ObjectConfidence: objectConfidence, // Is there anything here?
		ClassConfidence:  classConfidence,  // what class is likely here?
		Confidence:       finalConf,
		Rect:             rect,
	})

	return detections
}

func logRows(camera, direction, message string) {
	entry := map[string]interface{}{
		"time":      time.Now().Format(time.RFC3339),
		"camera":    camera,
		"direction": direction,
		"message":   message,
	}

	jsonData, err := json.MarshalIndent(entry, "", "  ") // pretty-print
	if err != nil {
		fmt.Println("Error marshaling rows:", err)
		return
	}

	if _, err := y5RowLogger.Write(append(jsonData, '\n')); err != nil {
		fmt.Println("Error writing to row log file:", err)
	}
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
		fmt.Println("Error writing to detection log file:", err)
	}
}
