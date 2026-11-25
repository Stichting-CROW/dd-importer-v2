package validator

import (
	"encoding/json"
	"time"
)

// Request is a feed configuration to validate. It mirrors a feeds row; the
// feed does not need to be saved.
type Request struct {
	SystemID       string            `json:"system_id"`
	FeedURL        string            `json:"feed_url"`
	FeedType       string            `json:"feed_type"`
	DataType       string            `json:"data_type"`
	Authentication json.RawMessage   `json:"authentication"`
	RequestHeaders map[string]string `json:"request_headers"`
}

type Status string

const (
	// StatusValid means every checked file complies and the importer can read the feed.
	StatusValid Status = "valid"
	// StatusInvalid means a file does not comply or the importer cannot read the feed.
	StatusInvalid Status = "invalid"
	// StatusUnreachable means a file could not be downloaded.
	StatusUnreachable Status = "unreachable"
	// StatusNotChecked means the feed was downloaded but no schema applies.
	StatusNotChecked Status = "not_checked"
	// StatusConfigError means the feed configuration itself is unusable.
	StatusConfigError Status = "config_error"
)

type Result struct {
	Status          Status `json:"status"`
	Valid           bool   `json:"valid"`
	Standard        string `json:"standard"`
	DeclaredVersion string `json:"declared_version"`
	// ImporterIssues are reasons the importer cannot read this feed, even when
	// the files themselves may comply with the standard.
	ImporterIssues []string      `json:"importer_issues"`
	Messages       []string      `json:"messages"`
	Files          []*FileResult `json:"files"`
	CheckedAt      time.Time     `json:"checked_at"`
	DurationMs     int64         `json:"duration_ms"`
}

type FileResult struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Reachable is false when the file could not be downloaded, including
	// non-200 responses, except for trip hours without trips (404 or 410).
	Reachable       bool         `json:"reachable"`
	HTTPStatus      int          `json:"http_status"`
	Schema          string       `json:"schema"`
	SchemaChecked   bool         `json:"schema_checked"`
	DeclaredVersion string       `json:"declared_version"`
	RecordsChecked  *int         `json:"records_checked"`
	Valid           bool         `json:"valid"`
	Messages        []string     `json:"messages"`
	ErrorCount      int          `json:"error_count"`
	ErrorGroups     []ErrorGroup `json:"error_groups"`
	// ErrorGroupsTruncated is set when there were more than maxErrorGroups groups.
	ErrorGroupsTruncated bool `json:"error_groups_truncated"`
}

// ErrorGroup bundles schema errors that occur at the same place in every
// record, such as a missing field in each vehicle.
type ErrorGroup struct {
	// Path is the JSON pointer with array indices replaced by *.
	Path    string `json:"path"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	// Example is the JSON pointer of the first occurrence.
	Example string `json:"example"`
}

func newFileResult(name, url string) *FileResult {
	return &FileResult{
		Name:        name,
		URL:         MaskURL(url),
		Messages:    []string{},
		ErrorGroups: []ErrorGroup{},
	}
}

// finish derives the overall status from the files and importer issues,
// unless a status was already set (configuration error, timeout, panic).
func (r *Result) finish(started time.Time) {
	r.DurationMs = time.Since(started).Milliseconds()
	if r.Status != "" {
		r.Valid = false
		return
	}

	unreachable, invalid, checked := false, false, false
	for _, file := range r.Files {
		if !file.Reachable {
			unreachable = true
		}
		if file.SchemaChecked {
			checked = true
		}
		if !file.Valid {
			invalid = true
		}
	}

	switch {
	case unreachable:
		r.Status = StatusUnreachable
	case invalid || len(r.ImporterIssues) > 0:
		r.Status = StatusInvalid
	case !checked:
		r.Status = StatusNotChecked
	default:
		r.Status = StatusValid
	}
	r.Valid = r.Status == StatusValid
}
