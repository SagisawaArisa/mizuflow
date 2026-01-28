package v1

import (
	"encoding/json"
	"mizuflow/pkg/constraints"
)

type FeatureFlag struct {
	Namespace string `json:"namespace"`
	Env       string `json:"env"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int    `json:"version"`  // feature version
	Revision  int64  `json:"revision"` // overall etcd revision
	Type      string `json:"type"`
}

type FeatureStrategy struct {
	DefaultValue string `json:"default_value"`
	Rules        []Rule `json:"rules"`
}

type Rule struct {
	Attribute string   `json:"attribute"`
	Operator  string   `json:"operator"`
	Values    []string `json:"value"`
	Result    string   `json:"result"`
}

type Message struct {
	Namespace string             `json:"namespace"`
	Env       string             `json:"env"`
	Key       string             `json:"key"`
	Value     string             `json:"value"`
	Type      string             `json:"type"`
	Version   int                `json:"version"`
	Revision  int64              `json:"revision"`
	Action    constraints.Action `json:"action"`
}

func (f *FeatureFlag) ToJSON() string {
	b, err := json.Marshal(f)
	if err != nil {
		panic("MizuFlow serialization failed" + err.Error())
	}
	return string(b)
}
