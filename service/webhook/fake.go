package webhook

import "github.com/khaledhikmat/vs-go/service/config"

type webhookService struct {
	CfgSvc config.IService
}

func NewFake(cfgsvc config.IService) IService {
	return &webhookService{
		CfgSvc: cfgsvc,
	}
}

func (svc *webhookService) Post(_ map[string]interface{}) error {
	return nil
}
