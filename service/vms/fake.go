package vms

import (
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/storage"
)

type victorService struct {
	CfgSvc     config.IService
	StorageSvc storage.IService
}

func NewFake(cfgsvc config.IService, storagesvc storage.IService) IService {
	return &victorService{
		CfgSvc:     cfgsvc,
		StorageSvc: storagesvc,
	}
}

func (svc *victorService) RetrieveClip(vmsId string, from, to int64) (string, error) {
	return "", nil
}
