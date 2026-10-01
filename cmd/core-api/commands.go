package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	efoydb "github.com/blinge12/efoy/db"
	"github.com/blinge12/efoy/internal/config"
	"github.com/blinge12/efoy/internal/iam"
	iampg "github.com/blinge12/efoy/internal/iam/postgres"
	"github.com/blinge12/efoy/internal/platform/postgres"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
)

// runMigrate applies the embedded goose migrations: core-api migrate [up|status].
func runMigrate(args []string) error {
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(efoydb.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch cmd {
	case "up":
		return goose.UpContext(ctx, db, "migrations")
	case "status":
		return goose.StatusContext(ctx, db, "migrations")
	default:
		return fmt.Errorf("unknown migrate command %q (want up or status)", cmd)
	}
}

// runCreateStaff creates a web portal account with a password and TOTP, and
// prints the one-time secrets for the new user.
func runCreateStaff(args []string) error {
	fs := flag.NewFlagSet("create-staff", flag.ContinueOnError)
	email := fs.String("email", "", "staff email (required)")
	name := fs.String("name", "", "full name (required)")
	role := fs.String("role", string(authz.RoleSuperAdmin), "role, e.g. SUPER_ADMIN, DISPATCHER, SUPPORT_AGENT, INSTITUTION_ADMIN, FLEET_OWNER")
	scope := fs.String("scope", string(authz.ScopeGlobal), "GLOBAL, INSTITUTION, FLEET or ZONE")
	scopeID := fs.String("scope-id", "", "institution, fleet or zone id for scoped roles")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *name == "" {
		fs.Usage()
		return errors.New("--email and --name are required")
	}
	grant := authz.Grant{Role: authz.Role(*role), Scope: authz.ScopeType(*scope)}
	if *scopeID != "" {
		id, err := uuid.Parse(*scopeID)
		if err != nil {
			return fmt.Errorf("--scope-id: %w", err)
		}
		grant.ScopeID = id
	}

	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	keys, err := iam.LoadKeyring(cfg.Auth, cfg.Env, clock.Real{}, log)
	if err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The password is generated rather than taken as a flag so it never lands
	// in shell history; the user should change it on first login.
	res, err := iam.CreateStaff(ctx, iampg.New(pool), keys.TOTP, iam.CreateStaffInput{
		Email:    *email,
		FullName: *name,
		Grant:    grant,
	})
	if err != nil {
		return err
	}
	fmt.Printf(`Created %s (%s) with role %s.

Hand these to the user over a secure channel; they are not shown again.

  Password:     %s
  TOTP secret:  %s
  TOTP URL:     %s

Add the TOTP URL (or secret) to an authenticator app, then sign in to the console.
`, *email, res.UserID, grant, res.GeneratedPassword, res.TOTPSecret, res.TOTPURL)
	return nil
}

// runGenKeys prints fresh key material for a new environment's secrets.
func runGenKeys() error {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privPEM, pubPEM, err := iam.EncodeKeyPair(priv)
	if err != nil {
		return err
	}
	totpKey := make([]byte, 32)
	if _, err := rand.Read(totpKey); err != nil {
		return err
	}
	fmt.Printf("# EFOY_AUTH__SIGNING_KEY (keep secret)\n%s\n# Public key (keep to set EFOY_AUTH__PREVIOUS_PUBLIC_KEY after the next rotation)\n%s\n# EFOY_AUTH__TOTP_KEY (keep secret; changing it invalidates every enrolled TOTP)\n%s\n",
		privPEM, pubPEM, base64.StdEncoding.EncodeToString(totpKey))
	return nil
}
