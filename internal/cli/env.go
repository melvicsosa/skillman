package cli

import (
	"context"
	"encoding/json"
	"io"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
	"github.com/melvicsosa/skillman/internal/app"
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
	svc := app.New(app.Deps{
		Agents:     agents.Source{},
		AgentRepo:  sqlite.NewAgentRepo(db),
		Skills:     sqlite.NewSkillRepo(db),
		Projects:   sqlite.NewProjectRepo(db),
		Scans:      sqlite.NewScanRepo(db),
		Inspector:  skillfs.Inspector{},
		Quarantine: skillfs.Mover{},
		DataDir:    dir,
	})
	return &env{dataDir: dir, db: db, svc: svc}, nil
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
