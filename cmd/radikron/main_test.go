package main

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/iomz/radikron"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

func TestConfig(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Error(err)
	}
	client, err := radiko.New("")
	if err != nil {
		log.Fatal(err)
	}
	ck := radikron.ContextKey("asset")
	asset, err := radikron.NewAsset(client)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), ck, asset)
	cfg, err := reloadConfig(ctx, "test/config-test.yml")
	if err != nil {
		t.Error(err)
	}

	if asset.OutputFormat != radigo.AudioFormatAAC {
		t.Errorf("%v => want %v", asset.OutputFormat, radigo.AudioFormatAAC)
	}

	if len(cfg.Rules) != 4 {
		t.Error("error parsing the rules")
	}

	got := len(asset.AvailableStations)
	nStations := 12
	if got != nStations {
		t.Errorf("asset.AvailableStations: %v => want %v", got, nStations)
	}
}
