package retry

type Strategy interface {
	// NextTime 有没有下一次，已经下次重试的时间戳
	NextTime(req Req) (int64, bool)
}
type StrategyFunc func(req Req) (int64, bool)

func (f StrategyFunc) NextTime(req Req) (int64, bool) {
	return f(req)
}

type Req struct {
	CheckTimes int
}

// Builder 重试策略构造器，通过解析配置文件的json
type Builder interface {
	Build(configStr string) (Strategy, error)
}
