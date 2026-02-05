package apikeyfx

import (
	"github.com/bitmagnet-io/bitmagnet/internal/apikey"
	"github.com/bitmagnet-io/bitmagnet/internal/database/dao"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"apikey",
		fx.Provide(
			func(p Params) Result {
				return Result{
					Service: lazy.New(func() (apikey.Service, error) {
						d, err := p.Dao.Get()
						if err != nil {
							return nil, err
						}
						return apikey.NewService(d), nil
					}),
				}
			},
		),
	)
}

type Params struct {
	fx.In
	Dao lazy.Lazy[*dao.Query]
}

type Result struct {
	fx.Out
	Service lazy.Lazy[apikey.Service]
}
