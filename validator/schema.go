package validator

import (
	"bytes"
	"deelfietsdashboard-importer/validator/schemas"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const maxErrorGroups = 50

var (
	compiledMu sync.Mutex
	compiled   = map[string]*jsonschema.Schema{}
	printer    = message.NewPrinter(language.English)
)

// schemaExists reports whether an embedded schema exists, for example
// "gbfs/2.3/free_bike_status.json".
func schemaExists(path string) bool {
	_, err := fs.Stat(schemas.FS, path)
	return err == nil
}

// loadSchema compiles an embedded schema once. Every schema gets its own
// compiler, because the MDS 1.x releases all use the same $id.
func loadSchema(path string) (*jsonschema.Schema, error) {
	compiledMu.Lock()
	defer compiledMu.Unlock()
	if schema, ok := compiled[path]; ok {
		return schema, nil
	}

	raw, err := schemas.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	url := "mem:///" + path
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(url, doc); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		return nil, err
	}
	compiled[path] = schema
	return schema, nil
}

// checkSchema validates doc against an embedded schema and stores the grouped
// errors on the file. With ignoreVersion, errors on /version are dropped
// because the declared version was already used to pick the schema.
func checkSchema(file *FileResult, schemaPath, label string, doc any, ignoreVersion bool) {
	file.Schema = label
	schema, err := loadSchema(schemaPath)
	if err != nil {
		file.Messages = append(file.Messages, fmt.Sprintf("Schema %s kon niet geladen worden: %v", label, err))
		file.Valid = false
		return
	}
	file.SchemaChecked = true

	err = schema.Validate(doc)
	if err == nil {
		file.Valid = true
		return
	}
	var validationError *jsonschema.ValidationError
	if !errors.As(err, &validationError) {
		file.Messages = append(file.Messages, fmt.Sprintf("Valideren mislukt: %v", err))
		file.Valid = false
		return
	}

	leaves := collectLeaves(validationError)
	if ignoreVersion {
		kept := leaves[:0]
		for _, l := range leaves {
			if pointer(l.err.InstanceLocation) != "/version" {
				kept = append(kept, l)
			}
		}
		leaves = kept
	}
	file.ErrorCount = len(leaves)
	file.ErrorGroups, file.ErrorGroupsTruncated = groupErrors(leaves)
	file.Valid = len(leaves) == 0
}

type leaf struct {
	err     *jsonschema.ValidationError
	message string
}

// collectLeaves returns the most specific errors. For a failed oneOf/anyOf
// only the branch with the fewest errors is followed, so a vehicle that almost
// matches one mode is not also reported against every other mode. A failed
// contains is reported once on the array instead of once per item.
func collectLeaves(e *jsonschema.ValidationError) []leaf {
	switch e.ErrorKind.(type) {
	case *kind.Contains, *kind.MinContains:
		message := e.ErrorKind.LocalizedString(printer)
		if causes := collectChildren(e); len(causes) > 0 {
			message += " (" + causes[0].message + ")"
		}
		return []leaf{{e, message}}
	}
	if len(e.Causes) == 0 {
		return []leaf{{e, e.ErrorKind.LocalizedString(printer)}}
	}
	switch e.ErrorKind.(type) {
	case *kind.OneOf, *kind.AnyOf:
		var best []leaf
		for _, cause := range e.Causes {
			leaves := collectLeaves(cause)
			if best == nil || len(leaves) < len(best) {
				best = leaves
			}
		}
		return best
	}
	return collectChildren(e)
}

func collectChildren(e *jsonschema.ValidationError) []leaf {
	var leaves []leaf
	for _, cause := range e.Causes {
		leaves = append(leaves, collectLeaves(cause)...)
	}
	return leaves
}

func groupErrors(leaves []leaf) ([]ErrorGroup, bool) {
	groups := map[string]*ErrorGroup{}
	var order []string
	for _, l := range leaves {
		path := collapsedPointer(l.err.InstanceLocation)
		key := path + "|" + l.err.SchemaURL + "|" + strings.Join(l.err.ErrorKind.KeywordPath(), "/")
		group, ok := groups[key]
		if !ok {
			group = &ErrorGroup{
				Path:    path,
				Message: l.message,
				Example: pointer(l.err.InstanceLocation),
			}
			groups[key] = group
			order = append(order, key)
		}
		group.Count++
	}

	result := make([]ErrorGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > maxErrorGroups {
		return result[:maxErrorGroups], true
	}
	return result, false
}

func pointer(location []string) string {
	if len(location) == 0 {
		return "/"
	}
	return "/" + strings.Join(location, "/")
}

func collapsedPointer(location []string) string {
	collapsed := make([]string, len(location))
	for i, part := range location {
		if isIndex(part) {
			collapsed[i] = "*"
		} else {
			collapsed[i] = part
		}
	}
	return pointer(collapsed)
}

func isIndex(part string) bool {
	if part == "" {
		return false
	}
	for _, c := range part {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
