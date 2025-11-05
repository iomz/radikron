package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/iomz/radikron"
	"github.com/iomz/radikron/internal/config"
	"github.com/yyoshiki41/go-radiko"
)

// reloadConfig loads configuration and applies it to the asset in the context
func reloadConfig(ctx context.Context, filename string) (*config.Config, error) {
	// Update current time
	radikron.CurrentTime = time.Now().In(radikron.Location)

	// Load configuration
	cfg, err := config.LoadConfig(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get asset from context and apply configuration
	asset := radikron.GetAsset(ctx)
	if asset == nil {
		return nil, fmt.Errorf("asset not found in context")
	}

	if err := cfg.ApplyToAsset(asset); err != nil {
		return nil, fmt.Errorf("failed to apply config to asset: %w", err)
	}

	return cfg, nil
}

// processStation checks and downloads programs for a station
func processStation(ctx context.Context, wg *sync.WaitGroup, stationID string, rules radikron.Rules) {
	// Skip if no rules match this station
	if !rules.HasRuleWithoutStationID() && !rules.HasRuleForStationID(stationID) {
		return
	}

	// Fetch weekly programs
	weeklyPrograms, err := radikron.FetchWeeklyPrograms(stationID)
	if err != nil {
		log.Printf("failed to fetch the %s program: %v", stationID, err)
		return
	}
	log.Printf("checking the %s program", stationID)

	// Process each program
	for _, p := range weeklyPrograms {
		if rules.HasMatch(stationID, p) {
			if err := radikron.Download(ctx, wg, p); err != nil {
				log.Printf("download failed: %s", err)
			}
		}
	}
}

// processStations processes all stations in the asset
func processStations(ctx context.Context, wg *sync.WaitGroup, asset *radikron.Asset, rules radikron.Rules) {
	for _, stationID := range asset.AvailableStations {
		processStation(ctx, wg, stationID, rules)
	}
}

// setNextFetchTime sets the next fetch time for the asset
func setNextFetchTime(asset *radikron.Asset) {
	if asset.NextFetchTime == nil {
		oneDayLater := radikron.CurrentTime.Add(radikron.OneDay * time.Hour)
		asset.NextFetchTime = &oneDayLater
	}
}

// run is the main event loop
func run(wg *sync.WaitGroup, configFileName string) {
	client, err := radiko.New("")
	if err != nil {
		log.Fatal(err)
	}

	ck := radikron.ContextKey("asset")

	for {
		// Create new asset
		asset, err := radikron.NewAsset(client)
		if err != nil {
			log.Fatal(err)
		}

		// Create context with asset
		ctx := context.WithValue(context.Background(), ck, asset)

		// Load and apply configuration
		cfg, err := reloadConfig(ctx, configFileName)
		if err != nil {
			log.Fatal(err)
		}

		// Process all stations
		processStations(ctx, wg, asset, cfg.Rules)

		// Wait for all downloads to complete
		log.Println("waiting for all the downloads to complete")
		wg.Wait()

		// Set next fetch time
		setNextFetchTime(asset)

		// Sleep until next fetch time
		log.Printf("fetching completed – sleeping until %v", asset.NextFetchTime)
		fetchTimer := time.NewTimer(time.Until(*asset.NextFetchTime))
		<-fetchTimer.C
	}
}

func main() {
	// Parse flags
	conf := flag.String("c", "config.yml", "the config.yml to use.")
	enableDebug := flag.Bool("d", false, "enable debug mode.")
	version := flag.Bool("v", false, "print version.")
	flag.Parse()

	// Print version
	if *version {
		bi, _ := debug.ReadBuildInfo()
		fmt.Printf("%v\n", bi.Main.Version)
		os.Exit(0)
	}

	// Enable debug logging
	if *enableDebug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Println("starting radikron")

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run main loop in goroutine
	wg := sync.WaitGroup{}
	go run(&wg, *conf)

	// Wait for signal
	<-quit

	// Finish downloads in progress
	log.Println("exit once all the downloads complete")
	wg.Wait()
	log.Println("exiting radikron")
}
