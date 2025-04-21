package orphan

import "github.com/khaledhikmat/vs-go/model"

type IService interface {
	Publish(cameras []model.Camera) error
	Subscribe() (<-chan []model.Camera, error)
	Unsubscribe() error
}
