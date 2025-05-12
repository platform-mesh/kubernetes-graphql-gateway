package kcp

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL = errors.New("invalid URL format")
)

func combineBaseURLAndPath(baseURLStr, pathURLStr string) (string, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return "", errors.Join(ErrInvalidURL, err)
	}

	pathURL, err := url.Parse(pathURLStr)
	if err != nil {
		return "", errors.Join(ErrInvalidURL, err)
	}

	if pathURLStr == "" {
		return baseURL.String() + "/", nil
	}

	path := pathURL.Path

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	finalURL := url.URL{
		Scheme: baseURL.Scheme,
		Host:   baseURL.Host,
		Path:   path,
	}

	return finalURL.String(), nil
}
