package apikeycmd

import (
	"fmt"

	"github.com/bitmagnet-io/bitmagnet/internal/apikey"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	APIKeyService lazy.Lazy[apikey.Service]
}

type Result struct {
	fx.Out
	Command *cli.Command `group:"commands"`
}

func New(p Params) (Result, error) {
	cmd := &cli.Command{
		Name:  "apikey",
		Usage: "Manage API key authentication",
		Subcommands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Display the current API key",
				Action: func(ctx *cli.Context) error {
					svc, err := p.APIKeyService.Get()
					if err != nil {
						return fmt.Errorf("failed to get API key service: %w", err)
					}

					keyInfo, err := svc.GetKey(ctx.Context)
					if err != nil {
						return fmt.Errorf("failed to get API key: %w", err)
					}

					fmt.Fprintln(ctx.App.Writer, "API Key:", keyInfo.Key)
					fmt.Fprintln(ctx.App.Writer, "Created:", keyInfo.CreatedAt.Format("2006-01-02 15:04:05 MST"))
					fmt.Fprintln(ctx.App.Writer, "Updated:", keyInfo.UpdatedAt.Format("2006-01-02 15:04:05 MST"))

					return nil
				},
			},
			{
				Name:  "rotate",
				Usage: "Generate a new API key (invalidates the old one)",
				Action: func(ctx *cli.Context) error {
					svc, err := p.APIKeyService.Get()
					if err != nil {
						return fmt.Errorf("failed to get API key service: %w", err)
					}

					keyInfo, err := svc.RotateKey(ctx.Context)
					if err != nil {
						return fmt.Errorf("failed to rotate API key: %w", err)
					}

					fmt.Fprintln(ctx.App.Writer, "API key rotated successfully!")
					fmt.Fprintln(ctx.App.Writer, "New API Key:", keyInfo.Key)
					fmt.Fprintln(ctx.App.Writer, "Updated:", keyInfo.UpdatedAt.Format("2006-01-02 15:04:05 MST"))
					fmt.Fprintln(ctx.App.Writer)
					fmt.Fprintln(ctx.App.Writer, "Note: The old API key is now invalid. Restart the webui or refresh the browser to use the new key.")

					return nil
				},
			},
		},
	}

	return Result{Command: cmd}, nil
}
