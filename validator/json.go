package validator

import (
	"sort"
	"strconv"
	"strings"
)

func valueAt(doc any, keys ...string) any {
	for _, key := range keys {
		object, ok := doc.(map[string]any)
		if !ok {
			return nil
		}
		doc = object[key]
	}
	return doc
}

func stringAt(doc any, keys ...string) string {
	value, _ := valueAt(doc, keys...).(string)
	return value
}

func arrayAt(doc any, keys ...string) []any {
	value, _ := valueAt(doc, keys...).([]any)
	return value
}

// countAt stores the length of the array at keys as the number of records.
func countAt(file *FileResult, doc any, keys ...string) int {
	records, ok := valueAt(doc, keys...).([]any)
	if !ok {
		return -1
	}
	count := len(records)
	file.RecordsChecked = &count
	return count
}

// discoveryFeeds reads the name to URL list of a GBFS gbfs.json at keys.
func discoveryFeeds(doc any, keys ...string) (map[string]string, bool) {
	list, ok := valueAt(doc, keys...).([]any)
	if !ok {
		return nil, false
	}
	feeds := map[string]string{}
	for _, item := range list {
		name := stringAt(item, "name")
		url := stringAt(item, "url")
		if name != "" && url != "" {
			feeds[name] = url
		}
	}
	return feeds, true
}

// firstLanguage returns the GBFS 1.x/2.x language key to use, preferring en.
func firstLanguage(doc any) string {
	data, ok := valueAt(doc, "data").(map[string]any)
	if !ok {
		return ""
	}
	if _, ok := data["en"]; ok {
		return "en"
	}
	languages := make([]string, 0, len(data))
	for language := range data {
		if _, ok := valueAt(data[language], "feeds").([]any); ok {
			languages = append(languages, language)
		}
	}
	sort.Strings(languages)
	if len(languages) == 0 {
		return ""
	}
	return languages[0]
}

// versionParts splits "2.3" or "1.2.0" into major and minor; ok is false
// when the version does not start with two numbers.
func versionParts(version string) (major, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}
