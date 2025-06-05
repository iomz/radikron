package radikron

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bogem/id3v2"
	"github.com/grafov/m3u8"
	"github.com/yyoshiki41/radigo"
)

var sem = make(chan struct{}, MaxConcurrency)

func Download(
	ctx context.Context,
	wg *sync.WaitGroup,
	prog *Prog,
) (err error) {
	asset := GetAsset(ctx)
	title := prog.Title
	start := prog.Ft
	var startTime, nextEndTime time.Time

	startTime, err = time.ParseInLocation(DatetimeLayout, start, Location)
	if err != nil {
		return fmt.Errorf("invalid start time format '%s': %s", start, err)
	}

	// the program is in the future
	if startTime.After(CurrentTime) {
		nextEndTime, err = time.ParseInLocation(DatetimeLayout, prog.To, Location)
		if err != nil {
			return fmt.Errorf("invalid end time format '%s': %s", start, err)
		}
		// update the next fetching time
		if asset.NextFetchTime == nil || asset.NextFetchTime.After(nextEndTime) {
			next := nextEndTime.Add(BufferMinutes * time.Minute)
			asset.NextFetchTime = &next
		}
		return nil
	}

	// the program is already to be downloaded
	if asset.Schedules.HasDuplicate(prog) {
		log.Printf("-skip duplicate [%s]%s (%s)", prog.StationID, title, start)
		return nil
	}
	asset.Schedules = append(asset.Schedules, prog)

	// the output config
	output, err := newOutputConfig(
		fmt.Sprintf(
			"%s_%s_%s",
			startTime.In(Location).Format(OutputDatetimeLayout),
			prog.StationID,
			title,
		),
		asset.OutputFormat,
	)
	if err != nil {
		return fmt.Errorf("failed to configure output: %s", err)
	}
	if err = output.SetupDir(); err != nil {
		return fmt.Errorf("failed to setup the output dir: %s", err)
	}
	if output.IsExist() {
		log.Printf("-skip already exists: %s", output.AbsPath())
		return nil
	}

	// fetch the recording m3u8 uri
	uri, err := timeshiftProgM3U8(ctx, prog)
	if err != nil {
		return fmt.Errorf(
			"playlist.m3u8 not available [%s]%s (%s): %s",
			prog.StationID,
			title,
			start,
			err,
		)
	}
	log.Printf("start downloading [%s]%s (%s): %s", prog.StationID, title, start, uri)
	prog.M3U8 = uri
	wg.Add(1)
	go downloadProgram(ctx, wg, prog, output)
	return nil
}

func buildM3U8RequestURI(prog *Prog) string {
	u, err := url.Parse(APIPlaylistM3U8)
	if err != nil {
		log.Fatal(err)
	}
	// set query parameters
	urlQuery := u.Query()
	params := map[string]string{
		"station_id": prog.StationID,
		"ft":         prog.Ft,
		"to":         prog.To,
		"l":          "15", // required?
	}
	for k, v := range params {
		urlQuery.Set(k, v)
	}
	u.RawQuery = urlQuery.Encode()

	return u.String()
}

func bulkDownload(list []string, output string) error {
	var errFlag bool
	var wg sync.WaitGroup

	for _, v := range list {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()

			var err error
			for i := 0; i < MaxRetryAttempts; i++ {
				sem <- struct{}{}
				err = downloadLink(link, output)
				<-sem
				if err == nil {
					break
				}
			}
			if err != nil {
				log.Printf("failed to download: %s", err)
				errFlag = true
			}
		}(v)
	}
	wg.Wait()

	if errFlag {
		return errors.New("lack of aac files")
	}
	return nil
}

func downloadLink(link, output string) error {
	resp, err := http.Get(link) //nolint:gosec,noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, fileName := filepath.Split(link)
	file, err := os.Create(filepath.Join(output, fileName))
	if err != nil {
		return err
	}

	_, err = io.Copy(file, resp.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// downloadProgram manages the download for the given program
// in a go routine and notify the wg when finished
func downloadProgram(
	ctx context.Context, // the context for the request
	wg *sync.WaitGroup, // the wg to notify
	prog *Prog, // the program metadata
	output *radigo.OutputConfig, // the file configuration
) {
	defer wg.Done()
	var err error

	chunklist, err := getChunklistFromM3U8(prog.M3U8)
	if err != nil {
		log.Printf("failed to get chunklist: %s", err)
		return
	}

	aacDir, err := tempAACDir()
	if err != nil {
		log.Printf("failed to create the aac dir: %s", err)
		return
	}
	defer os.RemoveAll(aacDir) // clean up

	if err = bulkDownload(chunklist, aacDir); err != nil {
		log.Printf("failed to download aac files: %s", err)
		return
	}

	concatedFile, err := radigo.ConcatAACFilesFromList(ctx, aacDir)
	if err != nil {
		log.Printf("failed to concat aac files: %s", err)
		return
	}

	switch output.AudioFormat() {
	case radigo.AudioFormatAAC:
		err = os.Rename(concatedFile, output.AbsPath())
	case radigo.AudioFormatMP3:
		err = radigo.ConvertAACtoMP3(ctx, concatedFile, output.AbsPath())
	default:
		err = fmt.Errorf("invalid file format")
	}

	if err != nil {
		log.Printf("failed to write the output file: %s", err)
		return
	}

	info, err := os.Stat(output.AbsPath())
	if err != nil {
		log.Printf("failed to stat the output file: %s", err)
		return
	}

	asset := GetAsset(ctx)
	if info.Size() < asset.MinimumOutputSize {
		log.Printf("the output file is too small: %v MB", float32(info.Size())/Kilobytes/Kilobytes)
		err = os.Remove(output.AbsPath())
		if err != nil {
			log.Printf("failed to remove the file: %v", err)
			return
		}
		next := time.Now().In(Location).Add(BufferMinutes * time.Minute)
		asset.NextFetchTime = &next
		log.Printf("removed the file, retry downloading at %v", next)
		return
	}

	err = writeID3Tag(output, prog)
	if err != nil {
		log.Printf("ID3v2: %v", err)
		return
	}

	// finish downloading the file
	log.Printf("+file saved: %s", output.AbsPath())
}

// getChunklist returns a slice of uri string.
func getChunklist(input io.Reader) ([]string, error) {
	playlist, listType, err := m3u8.DecodeFrom(input, true)
	if err != nil || listType != m3u8.MEDIA {
		return nil, err
	}
	p := playlist.(*m3u8.MediaPlaylist)

	var chunklist []string
	for _, v := range p.Segments {
		if v != nil {
			chunklist = append(chunklist, v.URI)
		}
	}
	return chunklist, nil
}

// getChunklistFromM3U8 returns a slice of url.
func getChunklistFromM3U8(uri string) ([]string, error) {
	resp, err := http.Get(uri) //nolint:gosec,noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return getChunklist(resp.Body)
}

// getRadicronPath gets the RADICRON_HOME path
func getRadicronPath(sub string) (string, error) {
	// If the environment variable RADICRON_HOME is set,
	// override working directory path.
	fullPath := os.Getenv(EnvRadicronHome)
	switch {
	case fullPath != "" && !filepath.IsAbs(fullPath):
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		fullPath = filepath.Join(wd, fullPath, sub)
	case fullPath == "":
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		fullPath = filepath.Join(wd, "radiko", sub)
	default:
		fullPath = filepath.Join(fullPath, sub)
	}
	return filepath.Clean(fullPath), nil
}

// getURI returns uri generated by parsing m3u8.
func getURI(input io.Reader) (string, error) {
	playlist, listType, err := m3u8.DecodeFrom(input, true)
	if err != nil || listType != m3u8.MASTER {
		return "", err
	}
	p := playlist.(*m3u8.MasterPlaylist)

	if p == nil || len(p.Variants) != 1 || p.Variants[0] == nil {
		return "", errors.New("invalid m3u8 format")
	}
	return p.Variants[0].URI, nil
}

// newOutputConfig prepares the outputdir
func newOutputConfig(fileBaseName, fileFormat string) (*radigo.OutputConfig, error) {
	fullPath, err := getRadicronPath("downloads")
	if err != nil {
		return nil, err
	}

	return &radigo.OutputConfig{
		DirFullPath:  fullPath,
		FileBaseName: fileBaseName,
		FileFormat:   fileFormat,
	}, nil
}

// tempAACDir creates a dir to store temporary aac files
func tempAACDir() (string, error) {
	fullPath, err := getRadicronPath("tmp")
	if err != nil {
		return "", err
	}

	aacDir, err := os.MkdirTemp(fullPath, "aac")
	if err != nil {
		return "", err
	}

	return aacDir, nil
}

// timeshiftProgM3U8 gets playlist.m3u8 for a Prog
func timeshiftProgM3U8(
	ctx context.Context,
	prog *Prog,
) (string, error) {
	asset := GetAsset(ctx)
	client := asset.DefaultClient
	var req *http.Request
	var err error

	areaID := asset.GetAreaIDByStationID(prog.StationID)

	device, ok := asset.AreaDevices[areaID]
	if !ok {
		device, err = asset.NewDevice(areaID)
		if err != nil {
			return "", err
		}
	}

	uri := buildM3U8RequestURI(prog)
	req, _ = http.NewRequest("POST", uri, http.NoBody)
	req = req.WithContext(ctx)
	headers := map[string]string{
		UserAgentHeader:       device.UserAgent,
		RadikoAreaIDHeader:    areaID,
		RadikoAuthTokenHeader: device.AuthToken,
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return getURI(resp.Body)
}

func writeID3Tag(output *radigo.OutputConfig, prog *Prog) error {
	tag, err := id3v2.Open(output.AbsPath(), id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("error while opening the output file: %s", err)
	}
	defer tag.Close()

	// Set tags
	tag.SetTitle(output.FileBaseName)
	tag.SetArtist(prog.Pfm)
	tag.SetAlbum(prog.Title)
	tag.SetYear(prog.Ft[:4])
	tag.AddCommentFrame(id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    ID3v2LangJPN,
		Description: prog.Info,
	})

	// write tag to the aac
	if err = tag.Save(); err != nil {
		return fmt.Errorf("error while saving a tag: %s", err)
	}

	return nil
}
