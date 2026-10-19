package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// experienceDataClient fetches an experience's own backing data
// server-side so it can be embedded in index.html as __EXPERIENCE_DATA__
// (the "SSR the data, not the markup" approach — see the Home dashboard).
// Modeled on wrcsClient (manifest.go): short timeout, and any failure is
// just "no data" rather than a failed page render — the client always has
// its own fetch fallback, so this is a pure perf optimization, never a
// hard dependency.
type experienceDataClient struct {
	http *http.Client
}

func newExperienceDataClient() *experienceDataClient {
	return &experienceDataClient{http: &http.Client{Timeout: 400 * time.Millisecond}}
}

// fetch returns the decoded JSON body for dataURL, or (nil, false) on any
// error/timeout/non-200/invalid-JSON — deliberately swallowed, see type
// doc above. Decoding (rather than passing the body through raw) means a
// malformed or malicious upstream response can't break out of the
// <script> tag it gets embedded in.
func (c *experienceDataClient) fetch(dataURL string) (interface{}, bool) {
	if dataURL == "" {
		return nil, false
	}
	resp, err := c.http.Get(dataURL)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false
	}
	return data, true
}
