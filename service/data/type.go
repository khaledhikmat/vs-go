package data

import "github.com/khaledhikmat/vs-go/model"

type IService interface {
	RetrieveCameras() ([]model.Camera, error)
	RetrieveCamerasByID(id string) (model.Camera, error)
	RetrieveCamerasByIDs(ids []string) ([]model.Camera, error)
	RetrieveOrphanedCameras(max int) ([]model.Camera, error)
	UpdateCameraExcluded(id string, excluded bool) error
	UpdateCameraAgentID(cameraID, agentID string) error
	UpdateCameraAgentHeartbeat(id string) error

	NewError(err interface{}) error
	NewAgentsManagerStats(stats model.AgentsManagerStats) error
	NewAgentStats(stats model.AgentStats) error
	NewFramerStats(stats model.FramerStats) error
	NewStreamerStats(stats model.StreamerStats) error
}
