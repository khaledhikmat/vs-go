package config

import "fmt"

type hardcodedService struct {
}

func NewHardCoded() IService {
	return &hardcodedService{}
}

func (svc *hardcodedService) GetModeMaxShutdownTime() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 5
}

func (svc *hardcodedService) GetInputFolder() string {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return "./settings"
}

func (svc *hardcodedService) GetCamerasInputFile() string {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return fmt.Sprintf("%s/cameras.json", svc.GetInputFolder())
}

func (svc *hardcodedService) GetRecordingsFolder() string {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return "./recordings"
}

func (svc *hardcodedService) GetMaxAgentsPerPod() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 1
}

func (svc *hardcodedService) GetAgentAlerterWebhookRetry() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 3 * 60 // 3 minutes
}

func (svc *hardcodedService) GetAgentPeriodicTimeout() int {

	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 30
}

func (svc *hardcodedService) GetAgentsManagerPeriodicTimeout() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 30
}

func (svc *hardcodedService) GetAgentsMonitorPeriodicTimeout() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 30
}

func (svc *hardcodedService) GetAgentsMonitorMaxOrphanedCameras() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 10
}
func (svc *hardcodedService) GetStreamerMaxWorkers() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 3
}
func (svc *hardcodedService) GetRecorderStreamerClipDuration() int {
	// For now, we are using a hardcoded value.
	// In the future, this should be read from a configuration file or environment variable.
	return 3
}
