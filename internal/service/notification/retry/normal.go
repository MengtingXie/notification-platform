package retry

import (
	"encoding/json"
	"time"
)

type NormalBuilder struct{}

type normalConfig struct {
	MaxRetryTimes int `json:"maxRetryTimes"`
	Interval      int `json:"interval"`
}

const (
	millSecond = 1000
)

func (n *NormalBuilder) Build(configStr string) (Strategy, error) {
	var cfg normalConfig
	err := json.Unmarshal([]byte(configStr), &cfg)
	if err != nil {
		return nil, err
	}
	return StrategyFunc(func(req Req) (int64, bool) {
		if req.CheckTimes >= cfg.MaxRetryTimes {
			return 0, false
		}
		return time.Now().UnixMilli() + int64(cfg.MaxRetryTimes*millSecond), true
	}), nil
}
