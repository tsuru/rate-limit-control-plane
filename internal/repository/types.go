package repository

type Data struct {
	Key    string `json:"key"`
	Zone   string `json:"zone"`
	Last   int64  `json:"last"`
	Excess int64  `json:"excess"`
}
