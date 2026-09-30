package radikron

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const radikoHTTPTimeout = 120 * time.Second
const radikoAreaEndpoint = "https://radiko.jp/area"

// NewRadikoHTTPClient returns an HTTP client with a fresh cookie jar and Radiko timeout.
func NewRadikoHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: http.DefaultTransport,
		Jar:       jar,
		Timeout:   radikoHTTPTimeout,
	}, nil
}

// currentAreaID detects the listener's Radiko area from the /area response.
func CurrentAreaID() (string, error) {
	client, err := NewRadikoHTTPClient()
	if err != nil {
		return "", err
	}
	return currentAreaIDWithClient(client, radikoAreaEndpoint)
}

func currentAreaIDWithClient(client *http.Client, endpoint string) (string, error) {
	if client == nil {
		return "", errors.New("HTTP client is nil")
	}
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("area lookup returned HTTP %s", resp.Status)
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", err
	}
	var areaID string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if areaID != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "span" && len(n.Attr) > 0 {
			areaID = n.Attr[0].Val
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if strings.TrimSpace(areaID) == "" {
		return "", errors.New("area ID not found in Radiko response")
	}
	return areaID, nil
}
