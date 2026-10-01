// Command core-api serves the Efoy REST API: IAM, drivers, vehicles,
// institutions, routes, subscriptions, pricing, trips, boarding, incidents,
// payments and admin endpoints.
//
// Usage:
//
//	core-api                  serve HTTP
//	core-api migrate [up|status]
//	core-api create-staff --email ... --name ... --role ...
//	core-api gen-keys         print new signing and TOTP keys
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/blinge12/efoy/internal/iam"
	iampg "github.com/blinge12/efoy/internal/iam/postgres"
	"github.com/blinge12/efoy/internal/notification"
	"github.com/blinge12/efoy/internal/platform/bootstrap"
	"github.com/blinge12/efoy/internal/platform/postgres"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
)

func main() {
	if len(os.Args) > 1 {
		var err error
		switch os.Args[1] {
		case "migrate":
			err = runMigrate(os.Args[2:])
		case "create-staff":
			err = runCreateStaff(os.Args[2:])
		case "gen-keys":
			err = runGenKeys()
		default:
			fmt.Fprintf(os.Stderr, "core-api: unknown command %q (want migrate, create-staff or gen-keys)\n", os.Args[1])
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "core-api %s: %v\n", os.Args[1], err)
			os.Exit(1)
		}
		return
	}
	bootstrap.Run("core-api", setup)
}

func setup(ctx context.Context, app *bootstrap.App) error {
	cfg := app.Config

	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	app.OnShutdown(pool.Close)
	app.Health.AddCheck("postgres", pool.Ping)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	app.OnShutdown(func() { _ = rdb.Close() })
	app.Health.AddCheck("redis", func(ctx context.Context) error { return rdb.Ping(ctx).Err() })

	if cfg.SMS.Provider != "console" {
		return fmt.Errorf("sms.provider %q is not supported yet (AfroMessage/GeezSMS arrive on day 5)", cfg.SMS.Provider)
	}

	clk := clock.Real{}
	keys, err := iam.LoadKeyring(cfg.Auth, cfg.Env, clk, app.Log)
	if err != nil {
		return err
	}
	svc := iam.NewService(iam.Deps{
		Repo:    iampg.New(pool),
		OTP:     iam.NewOTPStore(rdb, cfg.Auth.OTP, keys.OTPPepper),
		Limiter: iam.NewLimiter(rdb),
		Tokens:  keys.Tokens,
		TOTP:    keys.TOTP,
		SMS:     notification.NewConsoleSMS(app.Log),
		Clock:   clk,
		Config:  cfg.Auth,
		Log:     app.Log,
	})

	app.Router.Get("/.well-known/jwks.json", keys.Tokens.JWKS)
	app.Router.Route("/v1", func(r chi.Router) {
		r.Use(authz.Authenticate(keys.Tokens))
		iam.NewHandler(svc).Routes(r)
	})
	return nil
}
