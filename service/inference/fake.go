package inference

type fakeService struct {
}

func NewFake() IService {
	return &fakeService{}
}

func (svc *fakeService) Invoke(_ string, _ string) (Result, error) {
	return Result{
		FPS:           0,
		Score:         "0",
		AlertImageURL: "",
	}, nil
}

// Only 1 of out 10 frames is processed
// This is just a placeholder for the actual implementation
func (svc *fakeService) CanSkipFrame(frames int) bool {
	return frames%50 != 0
	// return false
}
