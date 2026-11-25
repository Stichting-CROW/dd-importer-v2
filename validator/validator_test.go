package validator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func serveJSON(w http.ResponseWriter, doc any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

// newServer serves the given documents by path; other paths return 404.
func newServer(t *testing.T, docs map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		doc, ok := docs[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if handler, ok := doc.(http.HandlerFunc); ok {
			handler(w, r)
			return
		}
		serveJSON(w, doc)
	}))
	t.Cleanup(server.Close)
	return server
}

func validate(t *testing.T, req Request) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return Validate(ctx, req)
}

func assertStatus(t *testing.T, result Result, want Status) {
	t.Helper()
	if result.Status != want {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("status = %s, want %s\n%s", result.Status, want, encoded)
	}
	if result.Valid != (want == StatusValid) {
		t.Fatalf("valid = %v for status %s", result.Valid, want)
	}
}

func assertIssue(t *testing.T, result Result, substring string) {
	t.Helper()
	for _, issue := range result.ImporterIssues {
		if strings.Contains(issue, substring) {
			return
		}
	}
	t.Fatalf("no importer issue containing %q in %q", substring, result.ImporterIssues)
}

func bikesCount(doc map[string]any) int {
	return len(doc["data"].(map[string]any)["bikes"].([]any))
}

func TestGBFSVehiclesValid(t *testing.T) {
	bikes := fixture(t, "gbfs/2.3/free_bike_status.json")
	server := newServer(t, map[string]any{"/free_bike_status.json": bikes})

	result := validate(t, Request{
		SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/free_bike_status.json",
	})

	assertStatus(t, result, StatusValid)
	file := result.Files[0]
	if file.Schema != "GBFS 2.3 free_bike_status.json" || *file.RecordsChecked != bikesCount(bikes) {
		t.Fatalf("unexpected file result %+v", file)
	}
	if result.Standard != "GBFS" || result.DeclaredVersion != "2.3" {
		t.Fatalf("standard %q version %q", result.Standard, result.DeclaredVersion)
	}
}

func TestGBFSVehiclesGroupsErrorsPerField(t *testing.T) {
	bikes := fixture(t, "gbfs/2.3/free_bike_status.json")
	for _, bike := range bikes["data"].(map[string]any)["bikes"].([]any) {
		bike.(map[string]any)["lat"] = "52.1"
	}
	server := newServer(t, map[string]any{"/fbs": bikes})

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/fbs"})

	assertStatus(t, result, StatusInvalid)
	groups := result.Files[0].ErrorGroups
	if len(groups) != 1 || groups[0].Path != "/data/bikes/*/lat" || groups[0].Count != bikesCount(bikes) {
		t.Fatalf("unexpected groups %+v", groups)
	}
	if groups[0].Example != "/data/bikes/0/lat" {
		t.Fatalf("example = %q", groups[0].Example)
	}
}

func TestGBFSVehicles3xIsAnImporterIssue(t *testing.T) {
	server := newServer(t, map[string]any{"/vehicle_status": fixture(t, "gbfs/3.0/vehicle_status.json")})

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/vehicle_status"})

	assertStatus(t, result, StatusInvalid)
	assertIssue(t, result, "GBFS 3.0 wordt niet geïmporteerd")
	if file := result.Files[0]; !file.Valid || file.Schema != "GBFS 3.0 vehicle_status.json" {
		t.Fatalf("the file itself complies with GBFS 3.0: %+v", file)
	}
}

func TestGBFSVehiclesWithoutVersion(t *testing.T) {
	bikes := fixture(t, "gbfs/2.3/free_bike_status.json")
	delete(bikes, "version")
	server := newServer(t, map[string]any{"/fbs": bikes})

	t.Run("operator with a fixed version in the importer", func(t *testing.T) {
		result := validate(t, Request{SystemID: "deelfietsnederland", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/fbs"})
		if len(result.ImporterIssues) != 0 || result.Files[0].Schema != "GBFS 2.0 free_bike_status.json" {
			t.Fatalf("unexpected result %+v / %+v", result, result.Files[0])
		}
	})

	t.Run("other operator", func(t *testing.T) {
		result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/fbs"})
		assertStatus(t, result, StatusInvalid)
		assertIssue(t, result, "vermeldt geen 'version'")
	})
}

// gbfsDiscovery returns the official gbfs.json fixture with its feed list
// replaced by feeds (in the 2.x or 3.x layout of the fixture).
func gbfsDiscovery(t *testing.T, version string, feeds map[string]string) map[string]any {
	doc := fixture(t, "gbfs/"+version+"/gbfs.json")
	list := []any{}
	feeds["system_information"] = "https://example.com/system_information"
	if strings.HasPrefix(version, "3") {
		feeds["vehicle_status"] = "https://example.com/vehicle_status"
	}
	for _, name := range []string{"system_information", "free_bike_status", "vehicle_status", "vehicle_types", "geofencing_zones"} {
		if url, ok := feeds[name]; ok {
			list = append(list, map[string]any{"name": name, "url": url})
		}
	}
	if strings.HasPrefix(version, "3") {
		doc["data"] = map[string]any{"feeds": list}
	} else {
		doc["data"] = map[string]any{"en": map[string]any{"feeds": list}}
	}
	return doc
}

func TestFullGBFSVehicles(t *testing.T) {
	docs := map[string]any{
		"/fbs": fixture(t, "gbfs/2.3/free_bike_status.json"),
		"/vt":  fixture(t, "gbfs/2.3/vehicle_types.json"),
	}
	server := newServer(t, docs)
	docs["/gbfs.json"] = gbfsDiscovery(t, "2.3", map[string]string{
		"free_bike_status": server.URL + "/fbs",
		"vehicle_types":    server.URL + "/vt",
	})

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "full_gbfs", FeedURL: server.URL + "/gbfs.json"})

	assertStatus(t, result, StatusValid)
	names := []string{}
	for _, file := range result.Files {
		names = append(names, file.Name)
	}
	if strings.Join(names, ",") != "gbfs.json,free_bike_status.json,vehicle_types.json" {
		t.Fatalf("checked files %v", names)
	}
}

func TestFullGBFSVehiclesWithoutFreeBikeStatus(t *testing.T) {
	docs := map[string]any{"/vt": fixture(t, "gbfs/2.3/vehicle_types.json")}
	server := newServer(t, docs)
	docs["/gbfs.json"] = gbfsDiscovery(t, "2.3", map[string]string{"vehicle_types": server.URL + "/vt"})

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "full_gbfs", FeedURL: server.URL + "/gbfs.json"})

	assertStatus(t, result, StatusInvalid)
	assertIssue(t, result, "free_bike_status")
}

func TestGBFSGeofencingViaManifest(t *testing.T) {
	docs := map[string]any{"/zones": fixture(t, "gbfs/3.0/geofencing_zones.json")}
	server := newServer(t, docs)
	docs["/gbfs.json"] = gbfsDiscovery(t, "3.0", map[string]string{"geofencing_zones": server.URL + "/zones"})
	manifest := fixture(t, "gbfs/3.0/manifest.json")
	manifest["data"] = map[string]any{"datasets": []any{map[string]any{
		"system_id": "utrecht",
		"versions": []any{
			map[string]any{"version": "2.3", "url": server.URL + "/old"},
			map[string]any{"version": "3.0", "url": server.URL + "/gbfs.json"},
		},
	}}}
	docs["/manifest.json"] = manifest

	result := validate(t, Request{SystemID: "test", DataType: "service_areas", FeedType: "manifest_gbfs", FeedURL: server.URL + "/manifest.json"})

	assertStatus(t, result, StatusValid)
	if len(result.Files) != 3 || result.Files[2].Name != "utrecht: geofencing_zones.json" {
		t.Fatalf("unexpected files %+v", result.Files)
	}
	if *result.Files[2].RecordsChecked == 0 {
		t.Fatal("expected zones to be counted")
	}
}

func TestMissingRequiredFeedIsReportedOnce(t *testing.T) {
	doc := fixture(t, "gbfs/2.3/gbfs.json")
	doc["data"] = map[string]any{"en": map[string]any{"feeds": []any{
		map[string]any{"name": "free_bike_status", "url": "https://example.com/fbs"},
		map[string]any{"name": "vehicle_types", "url": "https://example.com/vt"},
	}}}
	file := newFileResult("gbfs.json", "")
	checkGBFS(file, doc, "gbfs.json", "2.3")

	if file.Valid || len(file.ErrorGroups) != 1 || file.ErrorGroups[0].Path != "/data/en/feeds" {
		t.Fatalf("unexpected groups %+v", file.ErrorGroups)
	}
	if !strings.Contains(file.ErrorGroups[0].Message, "system_information") {
		t.Fatalf("message %q should name the missing feed", file.ErrorGroups[0].Message)
	}
}

func TestGBFSGeofencingDirect(t *testing.T) {
	server := newServer(t, map[string]any{"/zones": fixture(t, "gbfs/2.3/geofencing_zones.json")})

	result := validate(t, Request{SystemID: "test", DataType: "service_areas", FeedType: "gbfs", FeedURL: server.URL + "/zones"})

	assertStatus(t, result, StatusValid)
	if result.Files[0].Schema != "GBFS 2.3 geofencing_zones.json" {
		t.Fatalf("schema = %q", result.Files[0].Schema)
	}
}

func mdsV2Server(t *testing.T, vehicles, status map[string]any) *httptest.Server {
	return newServer(t, map[string]any{
		"/mds/vehicles": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("size") != "100000" {
				t.Errorf("expected the importer's size parameter, got %q", r.URL.RawQuery)
			}
			serveJSON(w, vehicles)
		}),
		"/mds/vehicles/status": status,
	})
}

func TestMDSV2Vehicles(t *testing.T) {
	server := mdsV2Server(t, fixture(t, "mds/2.0_vehicles.json"), fixture(t, "mds/2.0_vehicles_status.json"))

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "mds-v2", FeedURL: server.URL + "/mds/vehicles"})

	assertStatus(t, result, StatusValid)
	if len(result.Files) != 2 || result.Files[1].Schema != "MDS 2.0 provider /vehicles/status" {
		t.Fatalf("unexpected files %+v", result.Files)
	}
}

func TestMDSV2StatusWithUnknownDevice(t *testing.T) {
	vehicles := fixture(t, "mds/2.0_vehicles.json")
	vehicles["vehicles"].([]any)[0].(map[string]any)["device_id"] = "00000000-0000-4000-8000-000000000000"
	server := mdsV2Server(t, vehicles, fixture(t, "mds/2.0_vehicles_status.json"))

	result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "mds-v2", FeedURL: server.URL + "/mds/vehicles"})

	assertStatus(t, result, StatusInvalid)
	assertIssue(t, result, "negeert dan de hele feed")
}

func TestMDS1Vehicles(t *testing.T) {
	vehicles := fixture(t, "mds/1.2_vehicles.json")
	server := newServer(t, map[string]any{"/vehicles": vehicles})

	t.Run("1.2", func(t *testing.T) {
		result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "mds", FeedURL: server.URL + "/vehicles"})
		assertStatus(t, result, StatusValid)
	})

	t.Run("1.1 despite the upstream version pattern", func(t *testing.T) {
		vehicles["version"] = "1.1.0"
		result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "mds", FeedURL: server.URL + "/vehicles"})
		assertStatus(t, result, StatusValid)
		if result.Files[0].Schema != "MDS 1.1 provider /vehicles" {
			t.Fatalf("schema = %q", result.Files[0].Schema)
		}
	})

	t.Run("2.x declared for an MDS 1 feed type", func(t *testing.T) {
		vehicles["version"] = "2.0.0"
		result := validate(t, Request{SystemID: "test", DataType: "vehicles", FeedType: "mds", FeedURL: server.URL + "/vehicles"})
		assertIssue(t, result, "verwacht MDS 1.x")
	})
}

func TestMDSTripsWalksBackToAnHourWithTrips(t *testing.T) {
	lastHour := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	withTrips := lastHour.Add(-2 * time.Hour).Format("2006-01-02T15")
	empty := fixture(t, "mds/2.0_trips.json")
	empty["trips"] = []any{}
	server := newServer(t, map[string]any{"/trips": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("end_time") {
		case lastHour.Format("2006-01-02T15"):
			http.NotFound(w, r)
		case withTrips:
			serveJSON(w, fixture(t, "mds/2.0_trips.json"))
		default:
			serveJSON(w, empty)
		}
	})})

	result := validate(t, Request{SystemID: "test", DataType: "trips", FeedType: "mds-trips-v2", FeedURL: server.URL + "/trips"})

	assertStatus(t, result, StatusValid)
	file := result.Files[0]
	if file.Name != "/trips?end_time="+withTrips || *file.RecordsChecked != 1 {
		t.Fatalf("unexpected file %+v", file)
	}
}

func TestMDSTripsV1(t *testing.T) {
	server := newServer(t, map[string]any{"/trips": fixture(t, "mds/1.2_trips.json")})

	result := validate(t, Request{SystemID: "test", DataType: "trips", FeedType: "mds-trips-v1", FeedURL: server.URL + "/trips"})

	assertStatus(t, result, StatusValid)
}

func TestMDSTripsWithoutAnyTrips(t *testing.T) {
	server := newServer(t, map[string]any{})

	result := validate(t, Request{SystemID: "test", DataType: "trips", FeedType: "mds-trips-v2", FeedURL: server.URL + "/trips"})

	assertStatus(t, result, StatusNotChecked)
}

func TestTokenAuthentication(t *testing.T) {
	server := newServer(t, map[string]any{"/fbs": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" || r.Header.Get("X-Extra") != "1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		serveJSON(w, fixture(t, "gbfs/2.3/free_bike_status.json"))
	})})
	request := Request{
		SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/fbs",
		Authentication: json.RawMessage(`{"authentication_type":"token","ApiKeyName":"X-Api-Key","ApiKey":"secret"}`),
		RequestHeaders: map[string]string{"X-Extra": "1"},
	}

	assertStatus(t, validate(t, request), StatusValid)

	request.Authentication = json.RawMessage(`{"authentication_type":"token","ApiKeyName":"X-Api-Key","ApiKey":"wrong"}`)
	result := validate(t, request)
	assertStatus(t, result, StatusUnreachable)
	if result.Files[0].HTTPStatus != 401 || !strings.Contains(result.Files[0].Messages[0], "credentials") {
		t.Fatalf("unexpected file %+v", result.Files[0])
	}
}

func TestConfigErrors(t *testing.T) {
	cases := map[string]Request{
		"relative url":      {DataType: "vehicles", FeedType: "gbfs", FeedURL: "/free_bike_status.json"},
		"unsupported combo": {DataType: "trips", FeedType: "gbfs", FeedURL: "https://example.com"},
		"unknown data type": {DataType: "parking", FeedType: "gbfs", FeedURL: "https://example.com"},
		"incomplete token": {
			DataType: "vehicles", FeedType: "gbfs", FeedURL: "https://example.com",
			Authentication: json.RawMessage(`{"authentication_type":"token","ApiKeyName":"X-Api-Key"}`),
		},
		"unknown auth type": {
			DataType: "vehicles", FeedType: "gbfs", FeedURL: "https://example.com",
			Authentication: json.RawMessage(`{"authentication_type":"magic"}`),
		},
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			result := validate(t, request)
			assertStatus(t, result, StatusConfigError)
			if len(result.Files) != 0 {
				t.Fatal("nothing should be downloaded for an invalid configuration")
			}
		})
	}
}

func TestFailingOAuthTokenRequestDoesNotCrash(t *testing.T) {
	result := validate(t, Request{
		SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: "https://example.com/fbs",
		Authentication: json.RawMessage(`{"authentication_type":"oauth2","TokenURL":"http://127.0.0.1:1/token",
			"OAuth2Credentials":{"client_id":"a","client_secret":"b","grant_type":"client_credentials","scope":"c"}}`),
	})

	assertStatus(t, result, StatusUnreachable)
	if !strings.Contains(result.Messages[0], "OAuth-token") {
		t.Fatalf("messages %q", result.Messages)
	}
}

func TestValidationTimeout(t *testing.T) {
	server := newServer(t, map[string]any{"/slow": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result := Validate(ctx, Request{SystemID: "test", DataType: "vehicles", FeedType: "tomp", FeedURL: server.URL + "/slow"})

	assertStatus(t, result, StatusUnreachable)
}

func TestMaskURL(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/gbfs.json":                        "https://api.example.com/gbfs.json",
		"https://api.example.com/fbs?key=abc123&city=utrecht":      "https://api.example.com/fbs?key=•••&city=•••",
		"https://api.example.com/trips?end_time=2026-11-25T10":     "https://api.example.com/trips?end_time=2026-11-25T10",
		"https://user:pass@api.example.com/vehicles?size=100000":   "https://•••@api.example.com/vehicles?size=100000",
		"https://api.example.com/v?page%5Blimit%5D=5&token=secret": "https://api.example.com/v?page%5Blimit%5D=5&token=•••",
	}
	for input, want := range cases {
		if got := MaskURL(input); got != want {
			t.Errorf("MaskURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResultNeverContainsCredentials(t *testing.T) {
	server := newServer(t, map[string]any{})
	result := validate(t, Request{
		SystemID: "test", DataType: "vehicles", FeedType: "gbfs", FeedURL: server.URL + "/fbs?api_key=topsecret",
		Authentication: json.RawMessage(`{"authentication_type":"token","ApiKeyName":"X-Api-Key","ApiKey":"topsecret"}`),
	})

	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "topsecret") {
		t.Fatalf("credentials leaked into the result: %s", encoded)
	}
}
