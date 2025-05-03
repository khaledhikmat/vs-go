package config

// Streamer names
const (
	MP4RecorderName    = "mp4Recorder"
	SimpleDetectorName = "simpleDetector"
	Yolo5DetectorName  = "yolo5Detector"
)

type StreamerParameters struct {
	ClipDuration              int     `yaml:"clipDuration"`
	ModelPath                 string  `yaml:"modelPath"`
	CocoNamesPath             string  `yaml:"cocoNamesPath"`
	ObjectConfidenceThreshold float32 `yaml:"objectConfidenceThreshold"`
	ConfidenceThreshold       float32 `yaml:"confidenceThreshold"`
	CoolDownPeriod            int     `yaml:"coolDownPeriod"`
	Logging                   bool    `yaml:"logging"`
}

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
	GetStreamerParameters(name string) StreamerParameters
}
