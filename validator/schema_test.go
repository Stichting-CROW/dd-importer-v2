package validator

import (
	"deelfietsdashboard-importer/validator/schemas"
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedSchemasCompile(t *testing.T) {
	count := 0
	err := fs.WalkDir(schemas.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		count++
		if _, err := loadSchema(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 30 {
		t.Fatalf("expected at least 30 embedded schemas, found %d", count)
	}
}
