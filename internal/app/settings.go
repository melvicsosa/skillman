package app

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Settings keys stored in the settings table.
const (
	SettingPort          = "port"
	SettingGitHubToken   = "github_token"
	SettingSkillsSHToken = "skillssh_token"
	// SettingMarketplaces is a comma-separated list of "owner/repo" GitHub
	// repositories holding a .claude-plugin/marketplace.json.
	SettingMarketplaces = "marketplaces"
)

// DefaultMarketplaces is searched when the marketplaces setting is empty.
var DefaultMarketplaces = []string{"anthropics/claude-plugins-official"}

// DefaultPort is the port the UI listens on unless configured otherwise.
const DefaultPort = 3010

// SettingKeys lists every key `config get|set` accepts.
var SettingKeys = []string{SettingPort, SettingGitHubToken, SettingSkillsSHToken, SettingMarketplaces}

// Settings is the read view of the settings table. Tokens are write-only:
// only their presence is reported.
type Settings struct {
	Port             int  `json:"port"`
	HasGitHubToken   bool `json:"hasGithubToken"`
	HasSkillsSHToken bool `json:"hasSkillsshToken"`
	// Marketplaces lists the "owner/repo" marketplaces searched by
	// `search --source marketplace`.
	Marketplaces []string `json:"marketplaces"`
}

// SettingsPatch updates the fields that are non-nil. An empty token clears it.
type SettingsPatch struct {
	Port          *int      `json:"port"`
	GitHubToken   *string   `json:"githubToken"`
	SkillsSHToken *string   `json:"skillsshToken"`
	Marketplaces  *[]string `json:"marketplaces"`
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
	if out.Marketplaces, err = s.Marketplaces(ctx); err != nil {
		return Settings{}, err
	}
	return out, nil
}

// Marketplaces returns the configured marketplace repositories, or
// DefaultMarketplaces when none are set.
func (s *Service) Marketplaces(ctx context.Context) ([]string, error) {
	v, ok, err := s.settings.GetSetting(ctx, SettingMarketplaces)
	if err != nil {
		return nil, err
	}
	list := parseMarketplaces(v)
	if !ok || len(list) == 0 {
		return append([]string(nil), DefaultMarketplaces...), nil
	}
	return list, nil
}

// MarketplacesFrom is Marketplaces for callers that only hold the repository.
func MarketplacesFrom(ctx context.Context, settings domain.SettingsRepository) []string {
	v, ok, err := settings.GetSetting(ctx, SettingMarketplaces)
	if list := parseMarketplaces(v); err == nil && ok && len(list) > 0 {
		return list
	}
	return append([]string(nil), DefaultMarketplaces...)
}

var marketplaceRepoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func parseMarketplaces(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// validateMarketplaces checks every item is "owner/repo" and returns the
// normalized comma-separated value.
func validateMarketplaces(items []string) (string, error) {
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(strings.TrimPrefix(item, "https://github.com/"))
		if item == "" {
			continue
		}
		if !marketplaceRepoRE.MatchString(item) {
			return "", fmt.Errorf("marketplace %q: want owner/repo: %w", item, domain.ErrRejected)
		}
		out = append(out, item)
	}
	return strings.Join(out, ","), nil
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
	if p.Marketplaces != nil {
		v, err := validateMarketplaces(*p.Marketplaces)
		if err != nil {
			return Settings{}, false, err
		}
		if err := s.settings.SetSetting(ctx, SettingMarketplaces, v); err != nil {
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
	switch key {
	case SettingPort:
		p, err := s.Port(ctx)
		return strconv.Itoa(p), err
	case SettingMarketplaces:
		list, err := s.Marketplaces(ctx)
		return strings.Join(list, ","), err
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
	switch key {
	case SettingPort:
		p, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("port %q is not a number: %w", value, domain.ErrRejected)
		}
		if err := ValidatePort(p); err != nil {
			return err
		}
		value = strconv.Itoa(p)
	case SettingMarketplaces:
		v, err := validateMarketplaces(parseMarketplaces(value))
		if err != nil {
			return err
		}
		value = v
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
