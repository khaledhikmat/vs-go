package inference

type Result struct {
	FPS           int    `json:"fps"`
	Score         string `json:"score"`
	AlertImageURL string `json:"alertImageUrl"`
}

type IService interface {
	Invoke(modelName string, inputURL string) (Result, error)
}
