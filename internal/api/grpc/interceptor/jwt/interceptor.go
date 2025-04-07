package jwt

import (
	"context"
)

func GetBizIDFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(BizIDName)
	if val == nil {
		return 0, ErrBizIDNotFound
	}
	v, ok := val.(int64)
	if !ok {
		return 0, ErrBizIDNotFound
	}
	return v, nil
}
