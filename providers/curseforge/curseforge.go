package curseforge

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/patrickmn/go-cache"
	"github.com/sbaildon/wow-addon-downloader/providers"
	"golang.org/x/net/html"
)

func contains(s []html.Attribute, key string, value string) bool {
	for _, attribute := range s {
		if attribute.Key == key {
			if attribute.Val == value {
				return true
			}
			return false
		}
	}
	return false
}

func cacheFetch(u url.URL) (*http.Response, error) {
	resp, cacheHit := pageCache.Get(u.String())
	if !cacheHit {
		resp, err := http.Get(u.String())
		if err != nil {
			var nothing http.Response
			return &nothing, fmt.Errorf("Can't connect to %s", u.String())
		}
		pageCache.Add(u.String(), resp, cache.NoExpiration)
		return resp, nil
	}

	return resp.(*http.Response), nil
}

var pageCache *cache.Cache

func init() {
	providers.AddProvider("wow.curseforge.com", &CurseForge{})
	providers.AddProvider("www.wowace.com", &CurseForge{})
	pageCache = cache.New(cache.NoExpiration, cache.NoExpiration)
}

// CurseForge is a provider for curse
type CurseForge struct{}

// DownloadURL does something
func (p CurseForge) DownloadURL(u url.URL) (string, error) {
	u.Path = path.Join(u.Path, "/files/latest")
	return u.String(), nil
}

// GetName does something
func (p CurseForge) GetName(u url.URL) (string, error) {
	resp, err := cacheFetch(u)
	if err != nil {
		return "", err
	}

	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return "", fmt.Errorf("Unable to find name for %s", u.String())
			// return "hello", nil
		}

		if tt != html.SelfClosingTagToken {
			continue
		}

		t := z.Token()
		if t.Data != "meta" {
			continue
		}

		if !contains(t.Attr, "property", "og:title") {
			continue
		}

		for _, a := range t.Attr {
			if a.Key == "content" {
				return a.Val, nil
			}
		}
	}
}

// GetVersion does something
func (p CurseForge) GetVersion(u url.URL) (string, error) {
	resp, err := cacheFetch(u)
	if err != nil {
		return "", err
	}

	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()

		if tt == html.ErrorToken {
			return "", fmt.Errorf("Unable to find version for %s", u.String())
		}
		if tt != html.StartTagToken {
			continue
		}

		t := z.Token()
		if t.Data != "a" {
			continue
		}

		for _, a := range t.Attr {
			if a.Key == "data-name" {
				return a.Val, nil
			}
		}
	}
}
