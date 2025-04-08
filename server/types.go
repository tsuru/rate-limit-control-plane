package server

type Data struct {
	ID     string `json:"id"`
	Last   int64  `json:"last"`
	Excess int64  `json:"excess"`
}
