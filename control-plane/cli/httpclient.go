package main

import (
	"io"
	"net/http"
	"os"
)

func setAuthHeader(req *http.Request) {
	if key := os.Getenv("TINYAWS_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setAuthHeader(req)
	return http.DefaultClient.Do(req)
}

func httpPost(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	setAuthHeader(req)
	return http.DefaultClient.Do(req)
}

func httpDo(req *http.Request) (*http.Response, error) {
	setAuthHeader(req)
	return http.DefaultClient.Do(req)
}
