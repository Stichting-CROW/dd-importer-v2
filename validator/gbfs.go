package validator

import (
	"fmt"
	"strings"
)

// gbfsVersions are the GBFS versions with embedded schemas, oldest first.
var gbfsVersions = []string{"1.0", "1.1", "2.0", "2.1", "2.2", "2.3", "3.0"}

// gbfsVersionOverrides mirrors gbfs.getData: these operators publish no
// version, so the importer reads them as a fixed major version.
var gbfsVersionOverrides = map[string]int{
	"cykl":               1,
	"deelfietsnederland": 2,
}

const maxManifestSystems = 5

// gbfsSchema picks the schema for a GBFS file: the declared version when
// available, otherwise the newest earlier version of the same major.
func gbfsSchema(version, fileName string) (path, used string, exact, ok bool) {
	if schemaExists(fmt.Sprintf("gbfs/%s/%s", version, fileName)) {
		return fmt.Sprintf("gbfs/%s/%s", version, fileName), version, true, true
	}
	major, minor, valid := versionParts(version)
	if !valid {
		return "", "", false, false
	}
	for i := len(gbfsVersions) - 1; i >= 0; i-- {
		candidateMajor, candidateMinor, _ := versionParts(gbfsVersions[i])
		if candidateMajor != major || candidateMinor > minor {
			continue
		}
		path = fmt.Sprintf("gbfs/%s/%s", gbfsVersions[i], fileName)
		if schemaExists(path) {
			return path, gbfsVersions[i], false, true
		}
	}
	return "", "", false, false
}

func checkGBFS(file *FileResult, doc any, fileName, version string) {
	path, used, exact, ok := gbfsSchema(version, fileName)
	if !ok {
		file.Messages = append(file.Messages,
			fmt.Sprintf("Geen GBFS-schema voor %s in versie %s; alleen het downloaden is gecontroleerd.", fileName, version))
		file.Valid = true
		return
	}
	if !exact {
		file.Messages = append(file.Messages,
			fmt.Sprintf("Geen schema voor GBFS %s; gecontroleerd tegen GBFS %s.", version, used))
	}
	checkSchema(file, path, fmt.Sprintf("GBFS %s %s", used, fileName), doc, !exact)
}

// gbfsDeclaredVersion returns the version to pick a schema with. GBFS 1.0
// has no version field, so a missing version means 1.0.
func (r *run) gbfsDeclaredVersion(file *FileResult, doc any) string {
	declared := stringAt(doc, "version")
	file.DeclaredVersion = declared
	if r.result.DeclaredVersion == "" {
		r.result.DeclaredVersion = declared
	}
	if declared == "" {
		return "1.0"
	}
	return declared
}

// checkGBFSFile downloads one GBFS file and checks it against its schema.
func (r *run) checkGBFSFile(name, url, fileName string, countKeys ...string) (any, *FileResult) {
	file := r.addFile(name, url)
	doc, ok := r.downloadJSON(file, url)
	if !ok {
		return nil, file
	}
	checkGBFS(file, doc, fileName, r.gbfsDeclaredVersion(file, doc))
	if len(countKeys) > 0 {
		countAt(file, doc, countKeys...)
	}
	return doc, file
}

// checkGBFSVehicles checks a free_bike_status.json (1.x, 2.x) or
// vehicle_status.json (3.x) the way gbfs.getData reads it.
func (r *run) checkGBFSVehicles(name, url string) {
	file := r.addFile(name, url)
	doc, ok := r.downloadJSON(file, url)
	if !ok {
		return
	}

	declared := stringAt(doc, "version")
	schemaVersion := r.gbfsDeclaredVersion(file, doc)
	major, _, _ := versionParts(schemaVersion)
	if override, ok := gbfsVersionOverrides[r.feed.OperatorID]; ok {
		major = override
		if declared == "" {
			schemaVersion = fmt.Sprintf("%d.0", override)
			file.Messages = append(file.Messages,
				fmt.Sprintf("De feed vermeldt geen versie; de importer leest deze aanbieder altijd als GBFS %d.x.", override))
		}
	} else if declared == "" {
		file.Messages = append(file.Messages, "De feed vermeldt geen versie, dus geldt GBFS 1.0.")
		r.importerIssue(fmt.Sprintf(
			"%s vermeldt geen 'version'; de importer kan de GBFS-versie dan niet bepalen en importeert niets.", name))
	}

	var count int
	switch major {
	case 1, 2:
		checkGBFS(file, doc, "free_bike_status.json", schemaVersion)
		count = countAt(file, doc, "data", "bikes")
	case 3:
		r.importerIssue(fmt.Sprintf(
			"GBFS %s wordt niet geïmporteerd: de importer leest voor voertuigen alleen GBFS 1.x en 2.x (free_bike_status.json).",
			schemaVersion))
		checkGBFS(file, doc, "vehicle_status.json", schemaVersion)
		count = countAt(file, doc, "data", "vehicles")
	default:
		r.importerIssue(fmt.Sprintf("Onbekende GBFS-versie %q; de importer leest alleen GBFS 1.x en 2.x.", schemaVersion))
		file.Valid = false
		return
	}
	if count == 0 {
		file.Messages = append(file.Messages,
			"De feed bevat nu geen voertuigen; de importer telt een lege feed als mislukte import.")
	}
}

// checkGBFSFullVehicles follows gbfs.json like gbfs.ImportFullFeedVehicles,
// which only reads data.en.feeds and needs free_bike_status and vehicle_types.
func (r *run) checkGBFSFullVehicles() {
	doc, file := r.checkGBFSFile("gbfs.json", r.feed.Url, "gbfs.json")
	if doc == nil {
		return
	}

	feeds, ok := discoveryFeeds(doc, "data", "en", "feeds")
	if !ok {
		if v3Feeds, isV3 := discoveryFeeds(doc, "data", "feeds"); isV3 {
			r.importerIssue("gbfs.json heeft de indeling van GBFS 3.x; de importer leest voor voertuigen alleen data.en.feeds (GBFS 1.x en 2.x).")
			feeds = v3Feeds
		} else if language := firstLanguage(doc); language != "" {
			r.importerIssue(fmt.Sprintf(
				"gbfs.json heeft geen Engelstalige feedlijst (data.en.feeds) maar alleen %q; de importer leest alleen data.en.feeds.", language))
			feeds, _ = discoveryFeeds(doc, "data", language, "feeds")
		} else {
			file.Messages = append(file.Messages, "gbfs.json bevat geen lijst met feeds.")
			file.Valid = false
			return
		}
	}

	vehiclesURL := feeds["free_bike_status"]
	if vehiclesURL == "" {
		r.importerIssue("gbfs.json verwijst niet naar free_bike_status; de importer importeert dan geen voertuigen.")
		vehiclesURL = feeds["vehicle_status"]
	}
	if feeds["vehicle_types"] == "" {
		r.importerIssue("gbfs.json verwijst niet naar vehicle_types; de importer importeert dan geen voertuigen.")
	}

	if vehiclesURL != "" {
		r.checkGBFSVehicles(vehicleFileName(vehiclesURL), vehiclesURL)
	}
	if typesURL := feeds["vehicle_types"]; typesURL != "" {
		r.checkGBFSFile("vehicle_types.json", typesURL, "vehicle_types.json", "data", "vehicle_types")
	}
}

func vehicleFileName(url string) string {
	if strings.Contains(url, "vehicle_status") {
		return "vehicle_status.json"
	}
	return "free_bike_status.json"
}

// checkGBFSGeofencing follows a GBFS 3.x gbfs.json like
// gbfs.ImportFullGeofenceV3, which only reads data.feeds.
func (r *run) checkGBFSGeofencing(prefix, url string) {
	doc, file := r.checkGBFSFile(prefix+"gbfs.json", url, "gbfs.json")
	if doc == nil {
		return
	}

	feeds, ok := discoveryFeeds(doc, "data", "feeds")
	if !ok {
		language := firstLanguage(doc)
		if language == "" {
			file.Messages = append(file.Messages, "gbfs.json bevat geen lijst met feeds.")
			file.Valid = false
			return
		}
		r.importerIssue(prefix + "gbfs.json heeft de indeling van GBFS 1.x/2.x; de importer leest voor zones alleen data.feeds (GBFS 3.x).")
		feeds, _ = discoveryFeeds(doc, "data", language, "feeds")
	}

	zonesURL := feeds["geofencing_zones"]
	if zonesURL == "" {
		r.importerIssue(prefix + "gbfs.json verwijst niet naar geofencing_zones; de importer importeert dan geen zones.")
		return
	}
	r.checkGBFSFile(prefix+"geofencing_zones.json", zonesURL, "geofencing_zones.json", "data", "geofencing_zones", "features")
}

// checkGBFSManifest follows manifest.json like gbfs.LoadFeedsFromManifest,
// which imports every dataset version starting with 3.
func (r *run) checkGBFSManifest() {
	doc, file := r.checkGBFSFile("manifest.json", r.feed.Url, "manifest.json", "data", "datasets")
	if doc == nil {
		return
	}

	type system struct{ id, url string }
	var systems []system
	for _, dataset := range arrayAt(doc, "data", "datasets") {
		for _, version := range arrayAt(dataset, "versions") {
			if strings.HasPrefix(stringAt(version, "version"), "3") && stringAt(version, "url") != "" {
				systems = append(systems, system{stringAt(dataset, "system_id"), stringAt(version, "url")})
			}
		}
	}
	if len(systems) == 0 {
		r.importerIssue("manifest.json bevat geen GBFS 3.x-datasets; de importer importeert dan geen zones.")
		return
	}
	if len(systems) > maxManifestSystems {
		file.Messages = append(file.Messages, fmt.Sprintf(
			"De manifest bevat %d datasets; alleen de eerste %d zijn gecontroleerd.", len(systems), maxManifestSystems))
		systems = systems[:maxManifestSystems]
	}
	for _, s := range systems {
		r.checkGBFSGeofencing(s.id+": ", s.url)
	}
}
