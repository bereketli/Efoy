// Command core-api serves the Efoy REST API: IAM, drivers, vehicles,
// institutions, routes, subscriptions, pricing, trips, boarding, incidents,
// payments and admin endpoints.
package main

import (
	"context"

	"github.com/blinge12/efoy/internal/platform/bootstrap"
	"github.com/blinge12/efoy/internal/platform/postgres"
)

func main() {
	bootstrap.Run("core-api", func(ctx context.Context, app *bootstrap.App) error {
		pool, err := postgres.Connect(ctx, app.Config.Database)
		if err != nil {
			return err
		}
		app.OnShutdown(pool.Close)
		app.Health.AddCheck("postgres", pool.Ping)

		// Domain handlers generated from api/openapi/efoy.yaml are mounted on
		// app.Router under /v1 as each domain lands (auth from day 2).
		return nil
	})
}
