package main

import (
	"path/filepath"
	"testing"

	"github.com/0xballo/fui/internal/cli"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
)

func TestProfilesConfigRoundTrip(t *testing.T) {
	cfg := config.Defaults()
	cfg.ConfigFile = filepath.Join(t.TempDir(), "config.toml")
	p := profilesConfig{cfg: &cfg}

	in := lists.Profile{
		Name: "bank", DateColumn: "Date", AmountColumn: "Amount", DescriptionColumn: "Desc",
		Delimiter: ";", DateFormat: "02/01/2006", DecimalSeparator: ",", SkipRows: "2",
	}
	if err := p.SaveProfile(in); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	got, err := p.ListProfiles()
	if err != nil || len(got) != 1 || got[0] != in {
		t.Fatalf("ListProfiles = %+v, %v; want [%+v]", got, err, in)
	}
	if err := p.SaveProfile(lists.Profile{Name: "bad", SkipRows: "x"}); err == nil {
		t.Fatal("non-numeric skip rows should fail")
	}
	if err := p.DeleteProfile("bank"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if err := p.DeleteProfile("bank"); err == nil {
		t.Fatal("deleting a missing profile should fail")
	}
}

func TestSettingsListsOmitsUnwiredServices(t *testing.T) {
	cfg := config.Defaults()
	l := settingsLists(&cfg, &cli.Deps{})
	if l.Categories != nil || l.Rules != nil || l.Recurrences != nil || l.Profiles == nil {
		t.Fatalf("unexpected wiring: %+v", l)
	}
}
