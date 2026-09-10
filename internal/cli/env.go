package cli

import (
	"context"
	"encoding/json"
	"io"

	"os"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	"github.com/melvicsosa/skillman/internal/adapters/convert"
	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/registry/github"
	"github.com/melvicsosa/skillman/internal/adapters/registry/local"
	"github.com/melvicsosa/skillman/internal/adapters/registry/skillssh"
	"github.com/melvicsosa/skillman/internal/adapters/registry/urlsrc"
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// Settings keys and environment variables for registry tokens.
const (
	settingGitHubToken   = "github_token"
	settingSkillsSHToken = "skillssh_token"
	envGitHubToken       = "GITHUB_TOKEN"
	envSkillsSHToken     = "SKILLSSH_TOKEN"
)

// env is everything a command needs: the open database and the service.
type env struct {
	dataDir string
	db      *sqlite.DB
	svc     *app.Service
}

// newRuntime opens the database in the data dir and wires the service.
func newRuntime(ctx context.Context) (*env, error) {
	dir, err := resolveDataDir()
	if err != nil {
		return nil, err
	}
	db, err := sqlite.Open(ctx, dbPath(dir))
	if err != nil {
		return nil, err
	}
	settings := sqlite.NewSettingsRepo(db)
	gh := github.New(tokenFor(ctx, settings, settingGitHubToken, envGitHubToken))
	registries := []domain.Registry{
		skillssh.New(tokenFor(ctx, settings, settingSkillsSHToken, envSkillsSHToken), gh),
		urlsrc.New(),
		local.Source{Home: agents.Home},
		gh,
	}
	svc := app.New(app.Deps{
		Agents:     agents.Source{},
		AgentRepo:  sqlite.NewAgentRepo(db),
		Skills:     sqlite.NewSkillRepo(db),
		Projects:   sqlite.NewProjectRepo(db),
		Scans:      sqlite.NewScanRepo(db),
		Inspector:  skillfs.Inspector{},
		Quarantine: skillfs.Mover{},
		Vault:      sqlite.NewVaultRepo(db),
		Settings:   settings,
		Converter:  convert.Converter{},
		Registries: registries,
		DataDir:    dir,
	})
	return &env{dataDir: dir, db: db, svc: svc}, nil
}

// tokenFor reads a token from the settings table, falling back to env.
func tokenFor(ctx context.Context, settings domain.SettingsRepository, key, envKey string) string {
	if v, ok, err := settings.GetSetting(ctx, key); err == nil && ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

func (r *env) Close() error { return r.db.Close() }

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
