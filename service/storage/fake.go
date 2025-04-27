package storage

import "github.com/khaledhikmat/vs-go/service/config"

type s3Service struct {
	CfgSvc config.IService
}

func NewFake(cfgsvc config.IService) IService {
	return &s3Service{
		CfgSvc: cfgsvc,
	}
}

func (svc *s3Service) StoreFile(fileName string) (string, error) {
	return "", nil
}
