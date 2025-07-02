package process

import (
	"context"
	"database/sql"
	"deelfietsdashboard-importer/feed/gbfs"
	"deelfietsdashboard-importer/geoutil"
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

func (dataProcessor DataProcessor) processGeofence(feed gbfs.GBFSGeofencing) []Geofence {
	var featureCollection geojson.FeatureCollection

	err := featureCollection.UnmarshalJSON(feed.Data.GeofencingZones)
	if err.Error() == "geom: stride mismatch, got 3, want 2" {
		log.Printf("Removing third coordinate from GeoJSON for feed %s", feed.OperatorID)
		fixedGeoJSON, err := geoutil.RemoveThirdCoordinate(feed.Data.GeofencingZones)
		if err != nil {
			log.Printf("Error removing third coordinate: %v", err)
			return nil
		}
		featureCollection.UnmarshalJSON(fixedGeoJSON)
	} else if err != nil {
		log.Print("Other problem with deserializing FeatureCollection")
		log.Print(err)
	}

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
			geofence = dataProcessor.insertGeofence(obj, feed.OperatorID)
		}
		geofences = append(geofences, geofence)
	}
	return geofences
}

func (dataProcessor DataProcessor) insertGeofence(geometry wkb.Geom, operatorID string) Geofence {
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
