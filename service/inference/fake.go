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
