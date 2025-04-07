//go:build wireinject

package audit

import "github.com/google/wire"

func InitAudit() *Module {

	return &Module{
		Svc: nil,
	}
}

func InitMoudle() *Module {
	wire.Build(
		InitAudit,
	)
	return &Module{}
}
