package geoutil

import (
	"encoding/json"
)

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string                 `json:"type"`
	Geometry   Geometry               `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func RemoveThirdCoordinate(geojsonRaw json.RawMessage) (json.RawMessage, error) {
	var featureCollection FeatureCollection

	// Unmarshal the GeoJSON data
	err := json.Unmarshal(geojsonRaw, &featureCollection)
	if err != nil {
		return nil, err
	}

	// Iterate through features and modify coordinates
	for i, feature := range featureCollection.Features {
		switch feature.Geometry.Type {
		case "Point":
			var coords []float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &coords); err != nil {
				return nil, err
			}
			if len(coords) > 2 {
				coords = coords[:2]
			}
			featureCollection.Features[i].Geometry.Coordinates, _ = json.Marshal(coords)

		case "LineString":
			var coords [][]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &coords); err != nil {
				return nil, err
			}
			for j := range coords {
				if len(coords[j]) > 2 {
					coords[j] = coords[j][:2]
				}
			}
			featureCollection.Features[i].Geometry.Coordinates, _ = json.Marshal(coords)

		case "Polygon":
			var coords [][][]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &coords); err != nil {
				return nil, err
			}
			for j := range coords {
				for k := range coords[j] {
					if len(coords[j][k]) > 2 {
						coords[j][k] = coords[j][k][:2]
					}
				}
			}
			featureCollection.Features[i].Geometry.Coordinates, _ = json.Marshal(coords)

		case "MultiPolygon":
			var coords [][][][]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &coords); err != nil {
				return nil, err
			}
			for j := range coords {
				for k := range coords[j] {
					for l := range coords[j][k] {
						if len(coords[j][k][l]) > 2 {
							coords[j][k][l] = coords[j][k][l][:2]
						}
					}
				}
			}
			featureCollection.Features[i].Geometry.Coordinates, _ = json.Marshal(coords)
		}
	}

	// Marshal the modified FeatureCollection back to JSON
	modifiedGeoJSON, err := json.Marshal(featureCollection)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(modifiedGeoJSON), nil
}
