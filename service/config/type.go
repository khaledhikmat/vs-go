package config

type IService interface {
	GetModeMaxShutdownTime() int
	GetInputFolder() string
	GetCamerasInputFile() string
	GetRecordingsFolder() string
	GetMaxAgentsPerPod() int
	GetAgentAlerterPeriodicTimeout() int
	GetAgentPeriodicTimeout() int
	GetAgentsManagerPeriodicTimeout() int
	GetAgentsMonitorPeriodicTimeout() int
	GetAgentsMonitorMaxOrphanedCameras() int
	GetStreamerMaxWorkers() int
	GetRecorderStreamerClipDuration() int
	GetYolo5StreamerModelPath() string
	GetYolo5CocoNamesPath() string
	GetYolo5ConfidenceThreshold() float32
	IsYolo5Logging() bool
}
