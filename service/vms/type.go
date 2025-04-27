package vms

type IService interface {
	RetrieveClip(vmsID string, from, to int64) (string, error)
}
