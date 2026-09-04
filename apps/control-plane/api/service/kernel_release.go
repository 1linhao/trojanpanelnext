package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"trojan-panel/dao"
	"trojan-panel/model"
)

const (
	kernelReleaseCacheTTL     = 24 * time.Hour
	githubReleasePageSize     = 30
	githubReleaseResponseSize = 8 << 20
)

type KernelRelease struct {
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"publishedAt"`
	URL         string    `json:"url"`
}

type KernelReleaseCatalog struct {
	Kernel    string          `json:"kernel"`
	Channel   string          `json:"channel"`
	Releases  []KernelRelease `json:"releases"`
	FetchedAt time.Time       `json:"fetchedAt"`
	Stale     bool            `json:"stale"`
	Error     string          `json:"error,omitempty"`
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

func GetKernelReleases(ctx context.Context, kernel, channel string, refresh bool) (*KernelReleaseCatalog, error) {
	cache, cacheErr := dao.SelectKernelReleaseCache(kernel, channel)
	if cacheErr != nil && cacheErr != sql.ErrNoRows {
		return nil, cacheErr
	}
	if !refresh && cache != nil && time.Since(cache.FetchedAt) < kernelReleaseCacheTTL {
		return decodeReleaseCatalog(cache, false)
	}
	catalog, etag, notModified, err := fetchKernelReleases(ctx, kernel, channel, cache)
	if err != nil {
		if cache == nil {
			return nil, err
		}
		cache.Error = err.Error()
		_ = dao.UpsertKernelReleaseCache(*cache)
		stale, decodeErr := decodeReleaseCatalog(cache, true)
		if decodeErr != nil {
			return nil, err
		}
		stale.Error = err.Error()
		return stale, nil
	}
	if notModified && cache != nil {
		cache.FetchedAt = time.Now().UTC()
		cache.Error = ""
		if err = dao.UpsertKernelReleaseCache(*cache); err != nil {
			return nil, err
		}
		return decodeReleaseCatalog(cache, false)
	}
	catalog.FetchedAt = time.Now().UTC()
	payload, err := json.Marshal(catalog)
	if err != nil {
		return nil, err
	}
	if err = dao.UpsertKernelReleaseCache(model.KernelReleaseCache{
		Kernel: kernel, Channel: channel, Payload: string(payload),
		ETag: etag, FetchedAt: catalog.FetchedAt,
	}); err != nil {
		return nil, err
	}
	return catalog, nil
}

func fetchKernelReleases(ctx context.Context, kernel, channel string, cache *model.KernelReleaseCache) (*KernelReleaseCatalog, string, bool, error) {
	repository := "XTLS/Xray-core"
	if kernel == "hysteria2" {
		repository = "apernet/hysteria"
	} else if kernel != "xray" {
		return nil, "", false, errors.New("unsupported managed kernel")
	}
	endpoint := fmt.Sprintf(
		"https://api.github.com/repos/%s/releases?per_page=%d",
		repository,
		githubReleasePageSize,
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", false, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "trojan-panel")
	if cache != nil && cache.ETag != "" {
		request.Header.Set("If-None-Match", cache.ETag)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, "", false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return nil, response.Header.Get("ETag"), true, nil
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, "", false, fmt.Errorf("GitHub releases failed: %s: %s", response.Status, string(body))
	}
	var releases []githubRelease
	if err = json.NewDecoder(io.LimitReader(response.Body, githubReleaseResponseSize)).Decode(&releases); err != nil {
		return nil, "", false, err
	}
	return &KernelReleaseCatalog{
		Kernel: kernel, Channel: channel,
		Releases: filterKernelReleases(releases, channel, 10, kernel),
	}, response.Header.Get("ETag"), false, nil
}

func filterKernelReleases(releases []githubRelease, channel string, limit int, kernel string) []KernelRelease {
	prerelease := channel == "prerelease"
	result := make([]KernelRelease, 0, limit)
	for _, release := range releases {
		if release.Draft || release.Prerelease != prerelease {
			continue
		}
		version := release.TagName
		if kernel == "hysteria2" {
			if !strings.HasPrefix(version, "app/") {
				continue
			}
			version = strings.TrimPrefix(version, "app/")
		}
		result = append(result, KernelRelease{
			Version: version, Name: release.Name, Prerelease: release.Prerelease,
			PublishedAt: release.PublishedAt, URL: release.HTMLURL,
		})
		if len(result) == limit {
			break
		}
	}
	return result
}

func decodeReleaseCatalog(cache *model.KernelReleaseCache, stale bool) (*KernelReleaseCatalog, error) {
	var catalog KernelReleaseCatalog
	if err := json.Unmarshal([]byte(cache.Payload), &catalog); err != nil {
		return nil, err
	}
	catalog.FetchedAt = cache.FetchedAt
	catalog.Stale = stale
	catalog.Error = cache.Error
	return &catalog, nil
}
