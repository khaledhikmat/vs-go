package storage

type IService interface {
	StoreFile(fileName string) (string, error)
}
