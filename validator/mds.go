package validator

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// mdsVersions are the MDS versions with embedded schemas, oldest first.
var mdsVersions = []string{"1.0", "1.1", "1.2", "2.0"}

// The MDS 1.1.0 schemas still require version 1.0.x (upstream bug).
var mds11Version = regexp.MustCompile(`^1\.1\.[0-9]+$`)

// importedVehicleStates mirrors mdstwo.combineFeeds.
var importedVehicleStates = map[string]bool{
	"available": true, "non_operational": true, "reserved": true, "non_contactable": true,
}

const maxTripHoursBack = 24

// mdsSchema picks the schema for an MDS endpoint by the declared version,
// falling back to the newest earlier minor of the same major.
func mdsSchema(declared, fileName string, expectedMajor int) (path, used string, exact, ok bool) {
	major, minor, valid := versionParts(declared)
	if !valid {
		major, minor = expectedMajor, 99
	}
	for i := len(mdsVersions) - 1; i >= 0; i-- {
		candidateMajor, candidateMinor, _ := versionParts(mdsVersions[i])
		if candidateMajor != major || candidateMinor > minor {
			continue
		}
		path = fmt.Sprintf("mds/%s/%s", mdsVersions[i], fileName)
		if schemaExists(path) {
			return path, mdsVersions[i], valid && candidateMinor == minor, true
		}
	}
	return "", "", false, false
}

// checkMDS validates an MDS response. expectedMajor is the MDS major version
// the importer code for this feed type reads.
func (r *run) checkMDS(file *FileResult, doc any, fileName, endpoint string, expectedMajor int) {
	declared := stringAt(doc, "version")
	file.DeclaredVersion = declared
	if r.result.DeclaredVersion == "" {
		r.result.DeclaredVersion = declared
	}
	if major, _, ok := versionParts(declared); ok && major != expectedMajor {
		r.importerIssue(fmt.Sprintf(
			"Feedtype %s verwacht MDS %d.x, maar %s meldt versie %s.", r.feed.Type, expectedMajor, endpoint, declared))
	}

	path, used, exact, ok := mdsSchema(declared, fileName, expectedMajor)
	if !ok {
		file.Messages = append(file.Messages,
			fmt.Sprintf("Geen MDS-schema voor versie %q; alleen het downloaden is gecontroleerd.", declared))
		file.Valid = true
		return
	}
	if declared != "" && !exact {
		file.Messages = append(file.Messages, fmt.Sprintf("Geen schema voor MDS %s; gecontroleerd tegen MDS %s.", declared, used))
	}
	ignoreVersion := (declared != "" && !exact) || mds11Version.MatchString(declared)
	checkSchema(file, path, fmt.Sprintf("MDS %s provider %s", used, endpoint), doc, ignoreVersion)
}

// checkMDSVehicles checks an MDS 1.x /vehicles feed (feed type mds).
func (r *run) checkMDSVehicles() {
	file := r.addFile("/vehicles", r.feed.Url)
	doc, ok := r.downloadJSON(file, r.feed.Url)
	if !ok {
		return
	}
	r.checkMDS(file, doc, "vehicles.json", "/vehicles", 1)
	countAt(file, doc, "data", "vehicles")
	if stringAt(doc, "links", "next") != "" {
		file.Messages = append(file.Messages, "Alleen de eerste pagina is gecontroleerd.")
	}
}

// checkMDSV2Vehicles checks /vehicles and /vehicles/status with the same
// URLs as mdstwo.DownloadData.
func (r *run) checkMDSV2Vehicles() {
	vehiclesURL, statusURL, err := mdsV2URLs(r.feed.Url)
	if err != nil {
		r.configError("De feed-URL is ongeldig.")
		return
	}

	vehiclesFile := r.addFile("/vehicles", vehiclesURL)
	vehiclesDoc, vehiclesOK := r.downloadJSON(vehiclesFile, vehiclesURL)
	if vehiclesOK {
		r.checkMDS(vehiclesFile, vehiclesDoc, "vehicles.json", "/vehicles", 2)
		countAt(vehiclesFile, vehiclesDoc, "vehicles")
	}

	statusFile := r.addFile("/vehicles/status", statusURL)
	statusDoc, statusOK := r.downloadJSON(statusFile, statusURL)
	if statusOK {
		r.checkMDS(statusFile, statusDoc, "vehicles_status.json", "/vehicles/status", 2)
		countAt(statusFile, statusDoc, "vehicles_status")
	}

	if vehiclesOK && statusOK {
		r.checkMDSV2DeviceIDs(vehiclesDoc, statusDoc)
	}
}

// checkMDSV2DeviceIDs mirrors mdstwo.combineFeeds, which drops the whole feed
// when an imported status refers to a device that is not in /vehicles.
func (r *run) checkMDSV2DeviceIDs(vehiclesDoc, statusDoc any) {
	known := map[string]bool{}
	for _, vehicle := range arrayAt(vehiclesDoc, "vehicles") {
		known[stringAt(vehicle, "device_id")] = true
	}
	missing := 0
	example := ""
	for _, status := range arrayAt(statusDoc, "vehicles_status") {
		if !importedVehicleStates[stringAt(status, "last_event", "vehicle_state")] {
			continue
		}
		deviceID := stringAt(status, "device_id")
		if !known[deviceID] {
			missing++
			if example == "" {
				example = deviceID
			}
		}
	}
	if missing > 0 {
		r.importerIssue(fmt.Sprintf(
			"%d voertuigen in /vehicles/status staan niet in /vehicles (bijvoorbeeld device_id %s); de importer negeert dan de hele feed.",
			missing, example))
	}
}

func mdsV2URLs(feedURL string) (string, string, error) {
	withSize := func(raw string) (string, error) {
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("size", strconv.Itoa(100000))
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	vehiclesURL, err := withSize(feedURL)
	if err != nil {
		return "", "", err
	}
	statusURL, err := withSize(feedURL + "/status")
	return vehiclesURL, statusURL, err
}

// checkMDSTrips checks the newest complete hour that has trips, walking back
// up to maxTripHoursBack hours. The URL is built like mds(two).ImportTrips.
func (r *run) checkMDSTrips(expectedMajor int) {
	hour := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	var emptyFile *FileResult
	var emptyDoc any
	emptyHours := 0

	for i := 0; i < maxTripHoursBack; i++ {
		timestamp := hour.Add(-time.Duration(i) * time.Hour).Format("2006-01-02T15")
		tripsURL := fmt.Sprintf("%s?end_time=%s", r.feed.Url, timestamp)
		file := newFileResult("/trips?end_time="+timestamp, tripsURL)

		body, status, ok := r.downloadStatus(file, tripsURL)
		if !ok {
			r.result.Files = append(r.result.Files, file)
			return
		}
		if status == http.StatusNotFound || status == http.StatusGone {
			emptyHours++
			continue
		}
		if status != http.StatusOK {
			file.Messages = append(file.Messages, describeStatus(status))
			r.result.Files = append(r.result.Files, file)
			return
		}
		file.Reachable = true
		doc, ok := parseJSON(file, body)
		if !ok {
			r.result.Files = append(r.result.Files, file)
			return
		}
		keys := tripsKeys(expectedMajor)
		if len(arrayAt(doc, keys...)) == 0 {
			if emptyFile == nil {
				emptyFile, emptyDoc = file, doc
			}
			emptyHours++
			continue
		}

		if emptyHours > 0 {
			file.Messages = append(file.Messages, fmt.Sprintf(
				"De %d uur daarna bevatten geen ritten; dit is het meest recente uur met ritten.", emptyHours))
		}
		r.checkMDS(file, doc, "trips.json", "/trips", expectedMajor)
		countAt(file, doc, keys...)
		r.result.Files = append(r.result.Files, file)
		return
	}

	if emptyFile != nil {
		emptyFile.Messages = append(emptyFile.Messages, fmt.Sprintf(
			"Geen ritten gevonden in de laatste %d uur; alleen de structuur van een lege respons is gecontroleerd.", maxTripHoursBack))
		r.checkMDS(emptyFile, emptyDoc, "trips.json", "/trips", expectedMajor)
		countAt(emptyFile, emptyDoc, tripsKeys(expectedMajor)...)
		r.result.Files = append(r.result.Files, emptyFile)
		return
	}

	file := newFileResult("/trips", r.feed.Url)
	file.Reachable = true
	file.Valid = true
	file.HTTPStatus = http.StatusNotFound
	file.Messages = append(file.Messages, fmt.Sprintf(
		"Geen ritten in de laatste %d uur (HTTP 404 of 410 voor elk uur); er is niets om te controleren.", emptyHours))
	r.result.Files = append(r.result.Files, file)
}

func tripsKeys(expectedMajor int) []string {
	if expectedMajor == 1 {
		return []string{"data", "trips"}
	}
	return []string{"trips"}
}
