package services

import (
	"net/http"
	"time"
)

func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}
	return &http.Client{
		Transport: transport,
		// No global timeout here — callers supply context deadlines
	}
}
