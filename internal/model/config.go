package model

type ConfigItem struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Revision int64  `json:"revision"` // overall etcd revision
}
