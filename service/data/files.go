package data

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/config"
)

type filesDBService struct {
	CfgSvc config.IService
}

func NewFilesDB(cfgsvc config.IService) IService {
	return &filesDBService{
		CfgSvc: cfgsvc,
	}
}

func (svc *filesDBService) RetrieveCameras() ([]model.Camera, error) {
	cameras := []model.Camera{}

	input := svc.CfgSvc.GetCamerasInputFile()
	data, err := os.ReadFile(input)
	if err != nil {
		return cameras, err
	}

	cameras = make([]model.Camera, 100)
	err = json.Unmarshal([]byte(data), &cameras)
	if err != nil {
		return cameras, err
	}

	return cameras, nil
}

func (svc *filesDBService) RetrieveCamerasByID(id string) (model.Camera, error) {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return model.Camera{}, err
	}

	for _, camera := range cameras {
		if camera.ID == id {
			return camera, nil
		}
	}

	return model.Camera{}, nil
}

func (svc *filesDBService) RetrieveCamerasByIDs(ids []string) ([]model.Camera, error) {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return nil, err
	}

	var result []model.Camera
	for _, camera := range cameras {
		for _, id := range ids {
			if camera.ID == id {
				result = append(result, camera)
			}
		}
	}

	return result, nil
}

func (svc *filesDBService) RetrieveOrphanedCameras(max int) ([]model.Camera, error) {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return nil, err
	}

	var result []model.Camera
	now := time.Now().Unix()
	for _, camera := range cameras {
		if camera.AgentID == "" || now-camera.LastHeartBeat == 0 || (now-camera.LastHeartBeat > 5*60) {
			result = append(result, camera)
			if len(result) >= max {
				break
			}
		}
	}

	return result, nil
}

func (svc *filesDBService) UpdateCameraExcluded(id string, excluded bool) error {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return err
	}

	for i, camera := range cameras {
		if camera.ID == id {
			cameras[i].Excluded = excluded
			break
		}
	}

	data, err := json.Marshal(cameras)
	if err != nil {
		return err
	}

	output := svc.CfgSvc.GetCamerasInputFile()
	// Write the JSON data to the file (with truncation))
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (svc *filesDBService) UpdateCameraAgentID(cameraID, agentID string) error {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return err
	}

	for i, camera := range cameras {
		if camera.ID == cameraID {
			cameras[i].AgentID = agentID
			cameras[i].StartupTime = time.Now().Unix()
			cameras[i].LastHeartBeat = time.Now().Unix()
			cameras[i].Uptime = cameras[i].LastHeartBeat - cameras[i].StartupTime
			break
		}
	}

	data, err := json.Marshal(cameras)
	if err != nil {
		return err
	}

	output := svc.CfgSvc.GetCamerasInputFile()
	// Write the JSON data to the file (with truncation))
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (svc *filesDBService) UpdateCameraAgentHeartbeat(id string) error {
	cameras, err := svc.RetrieveCameras()
	if err != nil {
		return err
	}

	for i, camera := range cameras {
		if camera.ID == id {
			cameras[i].LastHeartBeat = time.Now().Unix()
			cameras[i].Uptime = cameras[i].LastHeartBeat - cameras[i].StartupTime
			break
		}
	}

	data, err := json.MarshalIndent(cameras, "", "  ")
	if err != nil {
		return err
	}

	output := svc.CfgSvc.GetCamerasInputFile()
	// Write the JSON data to the file (with truncation))
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (svc *filesDBService) NewError(err interface{}) error {
	// Determine if the error is custom
	var customErr model.CustomError
	if custom, ok := err.(model.CustomError); ok {
		customErr = custom
	} else {
		customErr.Processor = "N/A"
		customErr.Inner = err.(error)
		customErr.Message = err.(error).Error()
		customErr.StackTrace = "N/A"
		customErr.Misc = nil
	}

	// Create an error object to persist
	errorData := struct {
		Timestamp  int64                  `json:"timestamp"`
		Processor  string                 `json:"processor"`
		Inner      string                 `json:"innerError"`
		Message    string                 `json:"message"`
		StackTrace string                 `json:"stackTrace"`
		Misc       map[string]interface{} `json:"misc"`
	}{
		Timestamp:  time.Now().Unix(),
		Processor:  customErr.Processor,
		Inner:      customErr.Inner.Error(),
		Message:    customErr.Message,
		StackTrace: customErr.StackTrace,
		Misc:       customErr.Misc,
	}
	return newEntity(errorData, "errors", svc.CfgSvc)
}

func (svc *filesDBService) NewAgentsManagerStats(stats model.AgentsManagerStats) error {
	// Marshal the stats data to JSON
	stats.Timestamp = time.Now().Unix()
	return newEntity(stats, "agents-manager-stats", svc.CfgSvc)
}

func (svc *filesDBService) NewAgentStats(stats model.AgentStats) error {
	// Marshal the stats data to JSON
	stats.Timestamp = time.Now().Unix()
	return newEntity(stats, "agent-stats", svc.CfgSvc)
}

func (svc *filesDBService) NewFramerStats(stats model.FramerStats) error {
	// Marshal the stats data to JSON
	stats.Timestamp = time.Now().Unix()
	return newEntity(stats, "framer-stats", svc.CfgSvc)
}

func (svc *filesDBService) NewStreamerStats(stats model.StreamerStats) error {
	// Marshal the stats data to JSON
	stats.Timestamp = time.Now().Unix()
	return newEntity(stats, "streamer-stats", svc.CfgSvc)
}

func (svc *filesDBService) NewAlerterStats(stats model.AlerterStats) error {
	// Marshal the stats data to JSON
	stats.Timestamp = time.Now().Unix()
	return newEntity(stats, "alerter-stats", svc.CfgSvc)
}

func newEntity[T any](entity T, filename string, cfgsvc config.IService) error {
	entities, err := retrieveEntites[T](filename, cfgsvc)
	if err != nil {
		return err
	}

	entities = append(entities, entity)

	// Marshal the entity data to JSON
	data, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return err
	}

	// Open the file in append mode, create it if it doesn't exist
	file, fileErr := os.OpenFile(fmt.Sprintf("%s/%s.json", cfgsvc.GetInputFolder(), filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()

	// Write the JSON data to the file (with truncation))
	output := fmt.Sprintf("%s/%s.json", cfgsvc.GetInputFolder(), filename)
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func retrieveEntites[T any](filename string, cfgsvc config.IService) ([]T, error) {
	var entities []T

	data, err := os.ReadFile(fmt.Sprintf("%s/%s.json", cfgsvc.GetInputFolder(), filename))
	if err != nil {
		// WARNIG: File not found, return empty slice
		return entities, nil
	}

	entities = []T{}
	err = json.Unmarshal([]byte(data), &entities)
	if err != nil {
		return entities, err
	}

	// Unmarshal the JSON data into the slice of entities
	err = json.Unmarshal(data, &entities)
	if err != nil {
		return nil, err
	}

	return entities, nil
}
