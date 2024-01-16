package gbfs

import (
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"strings"

	"github.com/huandu/go-clone"
)

type GBFSManifest struct {
	Data struct {
		Datasets []struct {
			SystemID string `json:"system_id"`
			Versions []struct {
				URL     string `json:"url"`
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"datasets"`
	} `json:"data"`
	LastUpdated int    `json:"last_updated"`
	TTL         int    `json:"ttl"`
	Version     string `json:"version"`
}

func LoadFeedsFromManifest(f feed.Feed) []feed.Feed {
	var res []feed.Feed
	resp := f.DownloadData(f.Url)
	if resp == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	var gbfsManifest GBFSManifest
	decoder.Decode(&gbfsManifest)

	for _, dataset := range gbfsManifest.Data.Datasets {
		for _, version := range dataset.Versions {
			if strings.HasPrefix(version.Version, "3") {
				newFeed := clone.Clone(f).(feed.Feed)
				newFeed.Url = version.URL
				newFeed.Type = "full_gbfs"
				res = append(res, newFeed)
			}
		}
	}
	return res
}
