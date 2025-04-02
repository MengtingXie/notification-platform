package retry

import (
	"encoding/json"
	"fmt"
)

type facadeConfig struct {
	// 什么类型
	Type string `json:"type"`
}

type FacadeBuilder struct {
	builderMap map[string]Builder
}

func (f *FacadeBuilder) Build(configStr string) (Strategy, error) {
	var cfg facadeConfig
	err := json.Unmarshal([]byte(configStr), &cfg)
	if err != nil {
		return nil, err
	}
	retryStrategy, ok := f.builderMap[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("未知的重试策略：%s", cfg.Type)
	}
	return retryStrategy.Build(configStr)
}

func NewFacadeBuilder(builderMap map[string]Builder) Builder {
	return &FacadeBuilder{
		builderMap: builderMap,
	}
}
