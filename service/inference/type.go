package inference

type Result struct {
	FPS           int    `json:"fps"`
	Score         string `json:"score"`
	AlertImageURL string `json:"alertImageUrl"`
}

// There should be more input to CanSkipFrame than just frames
type IService interface {
	Invoke(modelName string, inputURL string) (Result, error)
	CanSkipFrame(frames int) bool
}
