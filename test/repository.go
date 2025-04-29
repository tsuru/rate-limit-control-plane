package test

type Repository struct {
	Body
	Header
}

type Body struct {
	Key    []byte
	Last   int64
	Excess int64
}

type Header struct {
}

type Repositories struct {
	Values map[string][]Repository
}

func NewRepository() *Repositories {
	return &Repositories{}
}

func (r *Repositories) GetRateLimit() map[string][]Repository {
	return r.Values
}

func (r *Repositories) SetRateLimit(zone string, values []Repository) {
	_, ok := r.Values[zone]
	if ok {
		r.Values[zone] = values
		return
	}
}
