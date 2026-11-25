package main

import (
	"deelfietsdashboard-importer/feed"
	"deelfietsdashboard-importer/feed/gbfs"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFeedServer serves a GBFS manifest, GBFS v3 overviews and geofencing
// files, some of them broken.
func newFeedServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	zones := `{"data":{"geofencing_zones":{"type":"FeatureCollection","features":[%s]}}}`
	feature := `{"type":"Feature","geometry":{"type":"Polygon","coordinates":[]},"properties":{}}`

	serve := func(path string, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, body)
		})
	}
	serve("/two_zones.json", fmt.Sprintf(zones, feature+","+feature))
	serve("/no_zones.json", fmt.Sprintf(zones, ""))
	serve("/gbfs_with_zones.json", fmt.Sprintf(
		`{"data":{"feeds":[{"name":"geofencing_zones","url":"%s/two_zones.json"}]}}`, server.URL))
	serve("/gbfs_without_zones.json", `{"data":{"feeds":[{"name":"vehicle_status","url":"x"}]}}`)
	serve("/manifest.json", fmt.Sprintf(`{"data":{"datasets":[
		{"system_id":"a","versions":[{"version":"3.0","url":"%[1]s/gbfs_with_zones.json"}]},
		{"system_id":"b","versions":[{"version":"3.0","url":"%[1]s/missing.json"}]}
	]}}`, server.URL))
	serve("/manifest_all_broken.json", fmt.Sprintf(`{"data":{"datasets":[
		{"system_id":"b","versions":[{"version":"3.0","url":"%s/missing.json"}]}
	]}}`, server.URL))
	return server
}

func TestDownloadFeed(t *testing.T) {
	server := newFeedServer(t)

	tests := []struct {
		name          string
		feedType      string
		path          string
		wantSupported bool
		wantZones     int
	}{
		{"gbfs with zones", "gbfs", "/two_zones.json", true, 2},
		{"gbfs without zones", "gbfs", "/no_zones.json", true, 0},
		{"gbfs not found", "gbfs", "/missing.json", true, 0},
		{"full_gbfs with zones", "full_gbfs", "/gbfs_with_zones.json", true, 2},
		{"full_gbfs without geofencing feed", "full_gbfs", "/gbfs_without_zones.json", true, 0},
		{"full_gbfs not found", "full_gbfs", "/missing.json", true, 0},
		{"manifest with one broken dataset", "manifest_gbfs", "/manifest.json", true, 2},
		{"manifest with only broken datasets", "manifest_gbfs", "/manifest_all_broken.json", true, 0},
		{"manifest not found", "manifest_gbfs", "/missing.json", true, 0},
		{"unsupported feed type", "mds", "/two_zones.json", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataFeed := feed.Feed{ID: 1, OperatorID: "op", Type: tt.feedType, Url: server.URL + tt.path}
			geofences, supported := downloadFeed(dataFeed)
			if supported != tt.wantSupported {
				t.Errorf("supported = %v, want %v", supported, tt.wantSupported)
			}
			if got := countZones(geofences); got != tt.wantZones {
				t.Errorf("zones = %d, want %d", got, tt.wantZones)
			}
		})
	}
}

func geofencing(zones string) gbfs.GBFSGeofencing {
	var res gbfs.GBFSGeofencing
	if zones != "" {
		res.Data.GeofencingZones = json.RawMessage(zones)
	}
	return res
}

func TestCountZones(t *testing.T) {
	oneZone := `{"type":"FeatureCollection","features":[{"type":"Feature"}]}`
	twoZones := `{"type":"FeatureCollection","features":[{"type":"Feature"},{"type":"Feature"}]}`

	tests := []struct {
		name      string
		geofences []gbfs.GBFSGeofencing
		want      int
	}{
		{"nothing downloaded", nil, 0},
		{"failed download", []gbfs.GBFSGeofencing{geofencing("")}, 0},
		{"empty feature collection", []gbfs.GBFSGeofencing{geofencing(`{"type":"FeatureCollection","features":[]}`)}, 0},
		{"not a feature collection", []gbfs.GBFSGeofencing{geofencing(`"oops"`)}, 0},
		{"one feed with zones", []gbfs.GBFSGeofencing{geofencing(twoZones)}, 2},
		{"manifest with one failing feed", []gbfs.GBFSGeofencing{geofencing(""), geofencing(oneZone)}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countZones(tt.geofences); got != tt.want {
				t.Errorf("countZones() = %d, want %d", got, tt.want)
			}
		})
	}
}
