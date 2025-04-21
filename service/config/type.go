package config

type IService interface {
	GetModeMaxShutdownTime() int
	GetInputFolder() string
	GetCamerasInputFile() string
	GetRecordingsFolder() string
	GetMaxAgentsPerPod() int
	GetAgentAlerterWebhookRetry() int
	GetAgentPeriodicTimeout() int
	GetAgentsManagerPeriodicTimeout() int
	GetAgentsMonitorPeriodicTimeout() int
	GetAgentsMonitorMaxOrphanedCameras() int
	GetStreamerMaxWorkers() int
	GetRecorderStreamerClipDuration() int
}
