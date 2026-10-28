package process

import (
	"context"
	"database/sql"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/geoutil"
	"encoding/json"
	"log"

	"github.com/lib/pq"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/geojson"
	"github.com/twpayne/go-geom/encoding/wkb"
)

func (dataProcessor DataProcessor) ProcessGeofences(data []gbfs.GBFSGeofencing) {
	geofencesPerOperator := map[string][]gbfs.GBFSGeofencing{}
	for _, feed := range data {
		geofencesPerOperator[feed.OperatorID] = append(geofencesPerOperator[feed.OperatorID], feed)
	}

	for operatorID, feeds := range geofencesPerOperator {
		dataProcessor.processGeofencesPerOperator(operatorID, feeds)
	}
}

func (dataProcessor DataProcessor) updateServiceArea(
	municipality string,
	operatorID string,
	serviceAreaGeometries []string,
) {
	tx, _ := dataProcessor.DB.BeginTx(context.Background(), nil)

	_, err := tx.Exec(
		`UPDATE service_area
		SET valid_until = NOW()
		WHERE municipality = $1 AND operator = $2 and valid_until IS NULL
		`, municipality, operatorID)
	if err != nil {
		log.Print(err)
	}
	_, err = tx.Exec(
		`INSERT INTO service_area 
			(municipality, operator, valid_from, service_area_geometries)
			VALUES ($1, $2, NOW(), $3)`, municipality, operatorID, pq.Array(serviceAreaGeometries))
	if err != nil {
		log.Print(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Print(err)
	}
}

// Modification needed to in the future support also situations where a municipality dissapears completely.
func (dataProcessor DataProcessor) processGeofencesPerOperator(operatorID string, feeds []gbfs.GBFSGeofencing) {
	var geofences []Geofence
	for _, feed := range feeds {
		log.Printf("Import geofences %s", feed.OperatorID)
		geofences = append(geofences, dataProcessor.processGeofence(feed)...)
	}

	serviceAreasPerMunicipality := getServiceAreaPerMunicipality(geofences)
	for municipality, serviceAreaGeometries := range serviceAreasPerMunicipality {
		// is default true, because if serviceArea doesn't exists this value should be true.
		serviceAreaIsChanged := true
		// Check if service_areas are changed.
		dataProcessor.DB.DB.QueryRow(
			`SELECT NOT (service_area_geometries @> $3 AND service_area_geometries <@ $3)
			FROM
			service_area
			WHERE municipality = $1 AND operator = $2 and valid_until IS NULL
			`, municipality, operatorID, pq.Array(serviceAreaGeometries)).Scan(&serviceAreaIsChanged)
		if serviceAreaIsChanged {
			dataProcessor.updateServiceArea(municipality, operatorID, serviceAreaGeometries)
		}
	}
}

type Geofence struct {
	GeometryHash   string
	Municipalities []string
}

type effectiveRules struct {
	RideStartAllowed   bool
	RideEndAllowed     bool
	RideThroughAllowed bool
}

func defaultRules() effectiveRules {
	return effectiveRules{
		RideStartAllowed:   true,
		RideEndAllowed:     true,
		RideThroughAllowed: true,
	}
}

func (r *effectiveRules) apply(rule gbfs.Rule) {
	if rule.RideStartAllowed != nil {
		r.RideStartAllowed = *rule.RideStartAllowed
	}
	if rule.RideEndAllowed != nil {
		r.RideEndAllowed = *rule.RideEndAllowed
	}
	if rule.RideThroughAllowed != nil {
		r.RideThroughAllowed = *rule.RideThroughAllowed
	}
}

func resolveRules(globalRules []gbfs.Rule, featureRules []gbfs.Rule) effectiveRules {
	rules := defaultRules()
	for _, rule := range globalRules {
		rules.apply(rule)
	}
	for _, rule := range featureRules {
		rules.apply(rule)
	}
	return rules
}

func isDisallowedRule(r effectiveRules) bool {
	return !r.RideStartAllowed || !r.RideEndAllowed
}

func shouldKeepFeature(f *geojson.Feature, globalRules []gbfs.Rule) bool {
	rulesRaw, ok := f.Properties["rules"]
	if !ok {
		return !isDisallowedRule(resolveRules(globalRules, nil))
	}

	rules, err := parseRules(rulesRaw)
	if err != nil {
		return true
	}

	return !isDisallowedRule(resolveRules(globalRules, rules))
}

func parseRules(v interface{}) ([]gbfs.Rule, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var rules []gbfs.Rule
	if err := json.Unmarshal(b, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

func filterFeaturesThatBlockBikes(features geojson.FeatureCollection, globalRules []gbfs.Rule) []*geojson.Feature {
	filtered := make([]*geojson.Feature, 0, len(features.Features))

	for _, f := range features.Features {
		if shouldKeepFeature(f, globalRules) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

func (dataProcessor DataProcessor) processGeofence(feed gbfs.GBFSGeofencing) []Geofence {
	var featureCollection geojson.FeatureCollection

	err := featureCollection.UnmarshalJSON(feed.Data.GeofencingZones)

	if err != nil && err.Error() == "geom: stride mismatch, got 3, want 2" {
		log.Printf("Removing third coordinate from GeoJSON for feed %s", feed.OperatorID)
		fixedGeoJSON, err := geoutil.RemoveThirdCoordinate(feed.Data.GeofencingZones)
		if err != nil {
			log.Printf("Error removing third coordinate: %v", err)
			return nil
		}
		if err := featureCollection.UnmarshalJSON(fixedGeoJSON); err != nil {
			log.Print("Other problem with deserializing FeatureCollection")
			log.Print(err)
			return nil
		}
	} else if err != nil {
		log.Print("Other problem with deserializing FeatureCollection")
		log.Print(err)
		return nil
	}

	featureCollection.Features = filterFeaturesThatBlockBikes(featureCollection, feed.Data.GlobalRules)

	var geofences []Geofence
	log.Printf("Lengte array: %d %s", len(featureCollection.Features), feed.OperatorID)
	for _, item := range featureCollection.Features {
		res, _ := geom.SetSRID(item.Geometry, 4326)
		obj := wkb.Geom{
			T: res,
		}

		q := dataProcessor.DB.QueryRow(
			`SELECT geom_hash, municipalities
			FROM service_area_geometry
			WHERE geom_hash = ENCODE(DIGEST($1::bytea, 'sha1'), 'hex')`,
			&obj)
		var geofence Geofence
		err := q.Scan(&geofence.GeometryHash, pq.Array(&geofence.Municipalities))
		if err == sql.ErrNoRows {
			log.Print("Inserting new geofence for operator " + feed.OperatorID)
			geofence = dataProcessor.insertGeofence(obj, feed.OperatorID)
		}

		// Only add geofence if it has a geometry hash (was inserted correctly and has municipalities)
		if geofence.GeometryHash != "" {
			geofences = append(geofences, geofence)
		}
	}
	return geofences
}

func (dataProcessor DataProcessor) insertGeofence(geometry wkb.Geom, operatorID string) Geofence {
	log.Print("Inserting new geofence for operator " + operatorID)
	result, err := dataProcessor.DB.Query(
		`SELECT municipality
		FROM zones 
		WHERE zone_type = 'municipality'
		AND ST_intersects(ST_MakeValid(st_SetSRID(ST_GeomFromWKB($1::bytea),4326)), area);`,
		&geometry)
	if err != nil {
		return Geofence{}
	}
	var municipalities []string
	for result.Next() {
		var gmcode string
		result.Scan(&gmcode)
		municipalities = append(municipalities, gmcode)

	}

	// If no municipalities found, do not insert geofence.
	if len(municipalities) == 0 {
		return Geofence{}
	}

	res := dataProcessor.DB.QueryRow(
		`INSERT INTO service_area_geometry (geom_hash, geom, municipalities)
		VALUES (ENCODE(DIGEST($1::bytea, 'sha1'), 'hex'),
			ST_MakeValid(ST_SetSRID(ST_GeomFromWKB($2::bytea), 4326)), $3)
		returning geom_hash`,
		&geometry, &geometry, pq.Array(municipalities))

	var geometryHash string
	err = res.Scan(&geometryHash)
	if err == sql.ErrNoRows {
		log.Printf("Something went wrong with INSERT query %+v", err)
		return Geofence{}
	}
	return Geofence{
		GeometryHash:   geometryHash,
		Municipalities: municipalities,
	}
}

func getServiceAreaPerMunicipality(geofences []Geofence) map[string][]string {
	geohashPerMunicipality := map[string][]string{}
	for _, geofence := range geofences {
		for _, municipality := range geofence.Municipalities {
			arrayPermunicipality, exists := geohashPerMunicipality[municipality]
			if !exists {
				geohashPerMunicipality[municipality] = []string{}
				arrayPermunicipality = geohashPerMunicipality[municipality]
			}
			geohashPerMunicipality[municipality] = append(arrayPermunicipality, geofence.GeometryHash)
		}

	}
	return geohashPerMunicipality
}
