package webhook

type IService interface {
	Post(payload map[string]interface{}) error
}
