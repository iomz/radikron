package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// extractBinary returns the contents of the top-level regular file named
// binName from a goreleaser .tar.gz or .zip archive. Only that one entry is
// read, so other paths in the archive are never written to disk.
func extractBinary(archiveName string, data []byte, binName string) ([]byte, error) {
	switch {
	case strings.HasSuffix(archiveName, ".tar.gz"):
		return extractFromTarGz(data, binName)
	case strings.HasSuffix(archiveName, ".zip"):
		return extractFromZip(data, binName)
	}
	return nil, fmt.Errorf("update: unsupported archive %s", archiveName)
}

func extractFromTarGz(data []byte, binName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("update: open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("update: read tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Clean(hdr.Name) == binName {
			return readLimited(tr)
		}
	}
	return nil, fmt.Errorf("update: %s not found in archive", binName)
}

func extractFromZip(data []byte, binName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("update: open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Mode().IsRegular() && path.Clean(f.Name) == binName {
			return readZipFile(f)
		}
	}
	return nil, fmt.Errorf("update: %s not found in archive", binName)
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("update: open %s: %w", f.Name, err)
	}
	defer rc.Close()
	return readLimited(rc)
}

// readLimited reads at most maxArchiveSize bytes, so a compressed entry
// cannot expand without bound.
func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("update: extract binary: %w", err)
	}
	if len(b) > maxArchiveSize {
		return nil, errors.New("update: extracted binary is too large")
	}
	if len(b) == 0 {
		return nil, errors.New("update: extracted binary is empty")
	}
	return b, nil
}
