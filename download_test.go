package radikron

import (
	"embed"
	"testing"
)

var (
	//go:embed test/playlist-test.m3u8
	PlaylistTestM3U8 embed.FS
)

func TestBuildM3U8RequestURI(t *testing.T) {
	prog := &Prog{
		StationID: "FMT",
		Ft:        "20230605130000",
		To:        "20230605145500",
	}
	uri := buildM3U8RequestURI(prog)
	want := "https://radiko.jp/v2/api/ts/playlist.m3u8?ft=20230605130000&l=15&station_id=FMT&to=20230605145500"
	if uri != want {
		t.Errorf("buildM3U8RequestURI => %v, want %v", uri, want)
	}
}

func TestGetURI(t *testing.T) {
	m3u8, err := PlaylistTestM3U8.Open("test/playlist-test.m3u8")
	if err != nil {
		t.Error(err)
	}
	uri, err := getURI(m3u8)
	if err != nil {
		t.Error(err)
	}
	want := "https://radiko.jp/v2/api/ts/chunklist/FsNE6Bt0.m3u8"
	if uri != want {
		t.Errorf("getURI => %v, want %v", uri, want)
	}
}
