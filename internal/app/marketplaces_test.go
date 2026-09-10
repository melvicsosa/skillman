package app

import (
	"context"
	"errors"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestMarketplacesSetting(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	list, err := f.svc.Marketplaces(ctx)
	if err != nil || len(list) != 1 || list[0] != DefaultMarketplaces[0] {
		t.Fatalf("default = %v %v", list, err)
	}
	if v, err := f.svc.ConfigGet(ctx, SettingMarketplaces); err != nil || v != DefaultMarketplaces[0] {
		t.Fatalf("config get = %q %v", v, err)
	}
	if err := f.svc.ConfigSet(ctx, SettingMarketplaces, "a/b, https://github.com/c/d ,"); err != nil {
		t.Fatal(err)
	}
	if list, _ = f.svc.Marketplaces(ctx); len(list) != 2 || list[0] != "a/b" || list[1] != "c/d" {
		t.Fatalf("after set = %v", list)
	}
	if err := f.svc.ConfigSet(ctx, SettingMarketplaces, "nope"); !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("bad value err = %v", err)
	}
	if err := f.svc.ConfigSet(ctx, SettingMarketplaces, ""); err != nil {
		t.Fatal(err)
	}
	if list, _ = f.svc.Marketplaces(ctx); len(list) != 1 || list[0] != DefaultMarketplaces[0] {
		t.Fatalf("cleared = %v", list)
	}
	if got := MarketplacesFrom(ctx, f.svc.settings); len(got) != 1 || got[0] != DefaultMarketplaces[0] {
		t.Fatalf("MarketplacesFrom = %v", got)
	}
}
