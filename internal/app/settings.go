package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Settings keys stored in the settings table.
const (
	SettingPort          = "port"
	SettingGitHubToken   = "github_token"
	SettingSkillsSHToken = "skillssh_token"
)

// DefaultPort is the port the UI listens on unless configured otherwise.
const DefaultPort = 3010

// SettingKeys lists every key `config get|set` accepts.
var SettingKeys = []string{SettingPort, SettingGitHubToken, SettingSkillsSHToken}

// Settings is the read view of the settings table. Tokens are write-only:
// only their presence is reported.
type Settings struct {
	Port             int  `json:"port"`
	HasGitHubToken   bool `json:"hasGithubToken"`
	HasSkillsSHToken bool `json:"hasSkillsshToken"`
}

// SettingsPatch updates the fields that are non-nil. An empty token clears it.
type SettingsPatch struct {
	Port          *int    `json:"port"`
	GitHubToken   *string `json:"githubToken"`
	SkillsSHToken *string `json:"skillsshToken"`
}

// ValidatePort rejects privileged and out-of-range ports.
func ValidatePort(p int) error {
	if p < 1024 || p > 65535 {
		return fmt.Errorf("port %d: must be between 1024 and 65535: %w", p, domain.ErrRejected)
	}
	return nil
}

// Port returns the configured port, or DefaultPort when unset or invalid.
func (s *Service) Port(ctx context.Context) (int, error) {
	v, ok, err := s.settings.GetSetting(ctx, SettingPort)
	if err != nil {
		return 0, err
	}
	if !ok || strings.TrimSpace(v) == "" {
		return DefaultPort, nil
	}
	p, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || ValidatePort(p) != nil {
		return DefaultPort, nil
	}
	return p, nil
}

// GetSettings returns the current settings.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	port, err := s.Port(ctx)
	if err != nil {
		return Settings{}, err
	}
	out := Settings{Port: port}
	if out.HasGitHubToken, err = s.hasSetting(ctx, SettingGitHubToken); err != nil {
		return Settings{}, err
	}
	if out.HasSkillsSHToken, err = s.hasSetting(ctx, SettingSkillsSHToken); err != nil {
		return Settings{}, err
	}
	return out, nil
}

func (s *Service) hasSetting(ctx context.Context, key string) (bool, error) {
	v, ok, err := s.settings.GetSetting(ctx, key)
	return ok && v != "", err
}

// UpdateSettings applies p and returns the new settings plus whether the
// port changed (callers decide if a running server needs a restart).
func (s *Service) UpdateSettings(ctx context.Context, p SettingsPatch) (Settings, bool, error) {
	portChanged := false
	if p.Port != nil {
		if err := ValidatePort(*p.Port); err != nil {
			return Settings{}, false, err
		}
		cur, err := s.Port(ctx)
		if err != nil {
			return Settings{}, false, err
		}
		portChanged = cur != *p.Port
		if err := s.settings.SetSetting(ctx, SettingPort, strconv.Itoa(*p.Port)); err != nil {
			return Settings{}, false, err
		}
	}
	if p.GitHubToken != nil {
		if err := s.settings.SetSetting(ctx, SettingGitHubToken, strings.TrimSpace(*p.GitHubToken)); err != nil {
			return Settings{}, false, err
		}
	}
	if p.SkillsSHToken != nil {
		if err := s.settings.SetSetting(ctx, SettingSkillsSHToken, strings.TrimSpace(*p.SkillsSHToken)); err != nil {
			return Settings{}, false, err
		}
	}
	out, err := s.GetSettings(ctx)
	return out, portChanged, err
}

// ConfigGet returns the raw value of key for the CLI. Token values are
// returned as stored; the CLI is a local, trusted surface.
func (s *Service) ConfigGet(ctx context.Context, key string) (string, error) {
	if !validSettingKey(key) {
		return "", fmt.Errorf("unknown setting %q (valid: %s): %w", key, strings.Join(SettingKeys, ", "), domain.ErrNotFound)
	}
	if key == SettingPort {
		p, err := s.Port(ctx)
		return strconv.Itoa(p), err
	}
	v, _, err := s.settings.GetSetting(ctx, key)
	return v, err
}

// ConfigSet validates and stores key=value for the CLI.
func (s *Service) ConfigSet(ctx context.Context, key, value string) error {
	if !validSettingKey(key) {
		return fmt.Errorf("unknown setting %q (valid: %s): %w", key, strings.Join(SettingKeys, ", "), domain.ErrNotFound)
	}
	value = strings.TrimSpace(value)
	if key == SettingPort {
		p, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("port %q is not a number: %w", value, domain.ErrRejected)
		}
		if err := ValidatePort(p); err != nil {
			return err
		}
		value = strconv.Itoa(p)
	}
	return s.settings.SetSetting(ctx, key, value)
}

func validSettingKey(key string) bool {
	for _, k := range SettingKeys {
		if k == key {
			return true
		}
	}
	return false
}
