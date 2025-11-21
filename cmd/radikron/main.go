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

// ProgramFetcher is an interface for fetching weekly programs
type ProgramFetcher interface {
	FetchWeeklyPrograms(stationID string) (radikron.Progs, error)
}

// Downloader is an interface for downloading programs
type Downloader interface {
	Download(ctx context.Context, wg *sync.WaitGroup, prog *radikron.Prog) error
}

// AssetCreator is a function type for creating assets
type AssetCreator func(client *radiko.Client) (*radikron.Asset, error)

// TimeProvider provides the current time
type TimeProvider func() time.Time

// radikronProgramFetcher implements ProgramFetcher using radikron.FetchWeeklyPrograms
type radikronProgramFetcher struct{}

func (f *radikronProgramFetcher) FetchWeeklyPrograms(stationID string) (radikron.Progs, error) {
	return radikron.FetchWeeklyPrograms(stationID)
}

// radikronDownloader implements Downloader using radikron.Download
type radikronDownloader struct{}

func (d *radikronDownloader) Download(ctx context.Context, wg *sync.WaitGroup, prog *radikron.Prog) error {
	return radikron.Download(ctx, wg, prog)
}

// reloadConfig loads configuration and applies it to the asset in the context
func reloadConfig(ctx context.Context, filename string, timeProvider TimeProvider) (*config.Config, error) {
	// Update current time using time provider
	radikron.CurrentTime = timeProvider().In(radikron.Location)

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
func processStation(
	ctx context.Context,
	wg *sync.WaitGroup,
	stationID string,
	rules radikron.Rules,
	fetcher ProgramFetcher,
	downloader Downloader,
) {
	// Skip if no rules match this station
	if !rules.HasRuleWithoutStationID() && !rules.HasRuleForStationID(stationID) {
		return
	}

	// Fetch weekly programs
	weeklyPrograms, err := fetcher.FetchWeeklyPrograms(stationID)
	if err != nil {
		log.Printf("failed to fetch the %s program: %v", stationID, err)
		return
	}
	log.Printf("checking the %s program", stationID)

	// Process each program
	for _, p := range weeklyPrograms {
		if matchedRule := rules.FindMatch(stationID, p); matchedRule != nil {
			p.RuleName = matchedRule.Name
			p.RuleFolder = matchedRule.Folder
			if err := downloader.Download(ctx, wg, p); err != nil {
				log.Printf("download failed: %s", err)
			}
		}
	}
}

// processStations processes all stations in the asset
func processStations(
	ctx context.Context,
	wg *sync.WaitGroup,
	asset *radikron.Asset,
	rules radikron.Rules,
	fetcher ProgramFetcher,
	downloader Downloader,
) {
	for _, stationID := range asset.AvailableStations {
		processStation(ctx, wg, stationID, rules, fetcher, downloader)
	}
}

// setNextFetchTime sets the next fetch time for the asset
func setNextFetchTime(asset *radikron.Asset, currentTime time.Time) {
	if asset.NextFetchTime == nil {
		oneDayLater := currentTime.Add(radikron.OneDay * time.Hour)
		asset.NextFetchTime = &oneDayLater
	}
}

// runIteration runs a single iteration of the main loop
func runIteration(
	ctx context.Context,
	wg *sync.WaitGroup,
	configFileName string,
	fetcher ProgramFetcher,
	downloader Downloader,
	timeProvider TimeProvider,
) error {
	// Load and apply configuration
	cfg, err := reloadConfig(ctx, configFileName, timeProvider)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Get asset from context
	asset := radikron.GetAsset(ctx)
	if asset == nil {
		return fmt.Errorf("asset not found in context")
	}

	// Process all stations
	processStations(ctx, wg, asset, cfg.Rules, fetcher, downloader)

	// Wait for all downloads to complete
	log.Println("waiting for all the downloads to complete")
	wg.Wait()

	// Set next fetch time
	setNextFetchTime(asset, timeProvider())

	return nil
}

// run is the main event loop
func run(
	wg *sync.WaitGroup,
	configFileName string,
	client *radiko.Client,
	assetCreator AssetCreator,
	fetcher ProgramFetcher,
	downloader Downloader,
	timeProvider TimeProvider,
	done <-chan struct{},
) {
	ck := radikron.ContextKey("asset")

	for {
		select {
		case <-done:
			return
		default:
		}

		// Create new asset
		asset, err := assetCreator(client)
		if err != nil {
			log.Fatal(err)
		}

		// Create context with asset
		ctx := context.WithValue(context.Background(), ck, asset)

		// Run single iteration
		if err := runIteration(ctx, wg, configFileName, fetcher, downloader, timeProvider); err != nil {
			log.Fatal(err)
		}

		// Sleep until next fetch time
		asset = radikron.GetAsset(ctx)
		if asset != nil && asset.NextFetchTime != nil {
			log.Printf("fetching completed – sleeping until %v", asset.NextFetchTime)
			fetchTimer := time.NewTimer(time.Until(*asset.NextFetchTime))
			select {
			case <-done:
				fetchTimer.Stop()
				return
			case <-fetchTimer.C:
			}
		}
	}
}

// runWithDefaults runs with default dependencies (for production use)
func runWithDefaults(wg *sync.WaitGroup, configFileName string, done <-chan struct{}) {
	client, err := radiko.New("")
	if err != nil {
		log.Fatal(err)
	}

	fetcher := &radikronProgramFetcher{}
	downloader := &radikronDownloader{}
	timeProvider := time.Now

	run(wg, configFileName, client, radikron.NewAsset, fetcher, downloader, timeProvider, done)
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

	// Create done channel for graceful shutdown
	done := make(chan struct{})

	// Run main loop in goroutine
	wg := sync.WaitGroup{}
	go func() {
		runWithDefaults(&wg, *conf, done)
	}()

	// Wait for signal
	<-quit
	close(done)

	// Finish downloads in progress
	log.Println("exit once all the downloads complete")
	wg.Wait()
	log.Println("exiting radikron")
}
