package gbfs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type GBFSVersion struct {
	Version string `json:"version"`
}

func GetMajorVersionFromResponse(data []byte) (int, error) {
	var version GBFSVersion
	err := json.Unmarshal(data, &version)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(version.Version, ".")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid version format")
	}
	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	return majorVersion, nil
}
