package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/iomz/radikron"
	"github.com/spf13/viper"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

// reload config to set a context and returns Rules
func reload(ctx context.Context, filename string) (radikron.Rules, error) {
	// update CurrentTime
	radikron.CurrentTime = time.Now().In(radikron.Location)

	// init Rules
	rules := radikron.Rules{}
	cwd, _ := os.Getwd()

	// check ${RADICRON_HOME}
	if os.Getenv("RADICRON_HOME") == "" {
		os.Setenv("RADICRON_HOME", filepath.Join(cwd, "radiko"))
	}

	// load params from a config file
	if filename != "config.yml" && filename != "config.toml" {
		configPath, err := filepath.Abs(filename)
		if err != nil {
			return rules, err
		}
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(cwd)
	}

	// read the config file
	if err := viper.ReadInConfig(); err != nil {
		return rules, fmt.Errorf("error reading config: %s", err)
	}

	// set the default area_id
	currentAreaID, err := radiko.AreaID()
	if err != nil {
		return rules, fmt.Errorf("error getting area-id: %s", err)
	}
	viper.SetDefault("area-id", currentAreaID)
	// set the default extra stations
	viper.SetDefault("extra-stations", []string{})
	// set the default ignore stations
	viper.SetDefault("ignore-stations", []string{})
	// set the default file-format as aac
	viper.SetDefault("file-format", radigo.AudioFormatAAC)
	// set the default minimum-output-size as 1MB
	viper.SetDefault("minimum-output-size", radikron.DefaultMinimumOutputSize)

	fileFormat := viper.GetString("file-format")

	// check the output file format
	if fileFormat != radigo.AudioFormatAAC &&
		fileFormat != radigo.AudioFormatMP3 {
		return rules, fmt.Errorf("unsupported audio format: %s", fileFormat)
	}
	// load the available station for AreaID
	areaID := viper.GetString("area-id")

	// extra/ignore stations
	extraStations := viper.GetStringSlice("extra-stations")
	ignoreStations := viper.GetStringSlice("ignore-stations")

	minimumOutputSize := viper.GetInt64("minimum-output-size")

	// save the asset in the current context
	asset := radikron.GetAsset(ctx)
	asset.OutputFormat = fileFormat
	asset.MinimumOutputSize = minimumOutputSize * radikron.Kilobytes * radikron.Kilobytes
	asset.LoadAvailableStations(areaID)
	asset.AddExtraStations(extraStations)
	asset.RemoveIgnoreStations(ignoreStations)

	// load rules from the file
	for name := range viper.GetStringMap("rules") {
		rule := &radikron.Rule{}
		err := viper.UnmarshalKey(fmt.Sprintf("rules.%s", name), rule)
		if err != nil {
			return rules, fmt.Errorf("error reading the rule: %s", err)
		}
		rule.SetName(name)
		// add the station-id to look up if not exists
		if rule.HasStationID() {
			isNewStation := true
			for _, as := range asset.AvailableStations {
				if as == rule.StationID {
					isNewStation = false
					break
				}
			}
			if isNewStation {
				asset.AddExtraStations([]string{rule.StationID})
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// run forever
func run(wg *sync.WaitGroup, configFileName string) {
	client, err := radiko.New("")
	if err != nil {
		log.Fatal(err)
	}
	ck := radikron.ContextKey("asset")
	for {
		// replenish asset
		asset, err := radikron.NewAsset(client)
		if err != nil {
			log.Fatal(err)
		}
		// new context with the asset
		ctx := context.WithValue(context.Background(), ck, asset)
		// reload config params
		rules, err := reload(ctx, configFileName)
		if err != nil {
			log.Fatal(err)
		}

		// check the weekly program for each station
		for _, stationID := range asset.AvailableStations {
			if !rules.HasRuleWithoutStationID() && // search all stations
				!rules.HasRuleForStationID(stationID) { // search this station
				continue
			}

			// fetch the weekly program
			weeklyPrograms, err := radikron.FetchWeeklyPrograms(stationID)
			if err != nil {
				log.Printf("failed to fetch the %s program: %v", stationID, err)
				continue
			}
			log.Printf("checking the %s program", stationID)

			// check each program
			for _, p := range weeklyPrograms {
				if rules.HasMatch(stationID, p) {
					err = radikron.Download(ctx, wg, p)
					if err != nil {
						log.Printf("downlod faild: %s", err)
					}
				}
			} // weeklyPrograms for stationID
		} // stations

		// wait for all the downloading jobs
		log.Println("waiting for all the downloads to complete")
		wg.Wait()

		// if the next program is not found, check again 24 hours later
		if asset.NextFetchTime == nil {
			oneDayLater := radikron.CurrentTime.Add(radikron.OneDay * time.Hour)
			asset.NextFetchTime = &oneDayLater
		}
		// sleep
		log.Printf("fetching completed – sleeping until %v", asset.NextFetchTime)
		// sleep until the next earliest program to be available
		fetchTimer := time.NewTimer(time.Until(*asset.NextFetchTime))
		<-fetchTimer.C
	}
}

func main() {
	// Set the config location
	conf := flag.String("c", "config.yml", "the config.yml to use.")
	enableDebug := flag.Bool("d", false, "enable debug mode.")
	version := flag.Bool("v", false, "print version.")
	flag.Parse()

	// use the version from build
	if *version {
		bi, _ := debug.ReadBuildInfo()
		fmt.Printf("%v\n", bi.Main.Version)
		os.Exit(0)
	}

	// to change the flags on the default logger
	if *enableDebug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Println("starting radikron")
	wg := sync.WaitGroup{}
	run(&wg, *conf)

	// listen for SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	// finish the downloading in progress
	log.Println("exit once all the downloads complete")
	wg.Wait()
	log.Println("exiting radikron")
}
