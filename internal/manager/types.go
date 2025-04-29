package manager

type Optional[T any] struct {
	Value T
	Error error
}

type Worker interface {
	Start()
	Stop()
	GetID() string
}
