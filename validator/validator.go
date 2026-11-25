// Package validator checks whether an operator feed complies with the data
// standard (MDS Vehicles, MDS Trips, GBFS) and whether the importer can read it.
// Files are downloaded with the importer's own authentication code.
package validator

import (
	"context"
	"deelfietsdashboard-importer/feed"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"slices"
	"time"
)

// FeedTypesByDataType lists the feed types each feeds.data_type supports,
// matching the importers (main.go, trip_importer, service_area_importer).
var FeedTypesByDataType = map[string][]string{
	"vehicles":      {"gbfs", "full_gbfs", "mds", "mds-v2", "tomp"},
	"trips":         {"mds-trips-v1", "mds-trips-v2"},
	"service_areas": {"gbfs", "full_gbfs", "manifest_gbfs"},
}

// Validate downloads and checks the feed. It returns when ctx is done, even
// if a request (such as an OAuth token request without timeout) still hangs.
func Validate(ctx context.Context, req Request) Result {
	started := time.Now()
	done := make(chan *Result, 1)

	go func() {
		r := &run{ctx: ctx, result: newResult(started)}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[%s] validation panicked (%T)", req.SystemID, recovered)
				r.result.Status = StatusUnreachable
				r.result.Messages = append(r.result.Messages,
					"De validatie is afgebroken door een fout bij het ophalen, waarschijnlijk bij het aanvragen van een OAuth-token. Controleer de credentials en de token-URL.")
			}
			done <- r.result
		}()
		r.validate(req)
	}()

	select {
	case result := <-done:
		result.finish(started)
		return *result
	case <-ctx.Done():
		result := newResult(started)
		result.Status = StatusUnreachable
		result.Messages = append(result.Messages, "De validatie duurde te lang en is afgebroken.")
		result.finish(started)
		return *result
	}
}

func newResult(started time.Time) *Result {
	return &Result{
		ImporterIssues: []string{},
		Messages:       []string{},
		Files:          []*FileResult{},
		CheckedAt:      started.UTC(),
	}
}

func (r *run) validate(req Request) {
	if !r.checkConfig(req) {
		return
	}

	switch req.DataType + "/" + req.FeedType {
	case "vehicles/gbfs":
		r.result.Standard = "GBFS"
		r.checkGBFSVehicles("free_bike_status.json", r.feed.Url)
	case "vehicles/full_gbfs":
		r.result.Standard = "GBFS"
		r.checkGBFSFullVehicles()
	case "vehicles/mds":
		r.result.Standard = "MDS"
		r.checkMDSVehicles()
	case "vehicles/mds-v2":
		r.result.Standard = "MDS"
		r.checkMDSV2Vehicles()
	case "vehicles/tomp":
		r.result.Standard = "TOMP"
		r.checkWithoutSchema("TOMP")
	case "trips/mds-trips-v1":
		r.result.Standard = "MDS"
		r.checkMDSTrips(1)
	case "trips/mds-trips-v2":
		r.result.Standard = "MDS"
		r.checkMDSTrips(2)
	case "service_areas/gbfs":
		r.result.Standard = "GBFS"
		r.checkGBFSFile("geofencing_zones.json", r.feed.Url, "geofencing_zones.json", "data", "geofencing_zones", "features")
	case "service_areas/full_gbfs":
		r.result.Standard = "GBFS"
		r.checkGBFSGeofencing("", r.feed.Url)
	case "service_areas/manifest_gbfs":
		r.result.Standard = "GBFS"
		r.checkGBFSManifest()
	}
}

func (r *run) checkConfig(req Request) bool {
	feedTypes, ok := FeedTypesByDataType[req.DataType]
	if !ok {
		r.configError(fmt.Sprintf("Onbekend datatype %q.", req.DataType))
		return false
	}
	if !slices.Contains(feedTypes, req.FeedType) {
		r.configError(fmt.Sprintf("Feedtype %q wordt niet ondersteund voor datatype %q.", req.FeedType, req.DataType))
		return false
	}
	u, err := url.Parse(req.FeedURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		r.configError("De feed-URL moet een volledige http(s)-URL zijn.")
		return false
	}

	if authenticationType := authenticationTypeOf(req.Authentication); authenticationType != "" &&
		!slices.Contains(feed.AuthenticationTypes, authenticationType) {
		r.configError(fmt.Sprintf("Onbekend authentication_type %q.", authenticationType))
		return false
	}
	r.feed = feed.Feed{
		OperatorID:     req.SystemID,
		Url:            req.FeedURL,
		Type:           req.FeedType,
		RequestHeaders: req.RequestHeaders,
	}
	if err := feed.ParseAuthentication(&r.feed, req.Authentication); err != nil {
		r.configError(fmt.Sprintf("Ongeldige authenticatie: %v.", err))
		return false
	}
	return true
}

func authenticationTypeOf(raw json.RawMessage) string {
	var config struct {
		AuthenticationType any `json:"authentication_type"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return ""
	}
	value, _ := config.AuthenticationType.(string)
	return value
}

// checkWithoutSchema only checks that the feed downloads and is JSON.
func (r *run) checkWithoutSchema(standard string) {
	file := r.addFile("feed", r.feed.Url)
	if _, ok := r.downloadJSON(file, r.feed.Url); !ok {
		return
	}
	file.Valid = true
	file.Messages = append(file.Messages, fmt.Sprintf(
		"Voor %s is geen schemacontrole beschikbaar; alleen gecontroleerd dat de feed bereikbaar is en geldige JSON teruggeeft.", standard))
}

func (r *run) addFile(name, url string) *FileResult {
	file := newFileResult(name, url)
	r.result.Files = append(r.result.Files, file)
	return file
}

func (r *run) importerIssue(issue string) {
	r.result.ImporterIssues = append(r.result.ImporterIssues, issue)
}

func (r *run) configError(message string) {
	r.result.Status = StatusConfigError
	r.result.Messages = append(r.result.Messages, message)
}
