package process

import (
	"database/sql"
	"deelfietsdashboard-importer/feed/gbfs"
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

func (dataProcessor DataProcessor) processGeofencesPerOperator(operatorID string, feeds []gbfs.GBFSGeofencing) {
	for _, feed := range feeds {
		dataProcessor.processGeofence(feed)
	}
}

func (dataProcessor DataProcessor) processGeofence(feed gbfs.GBFSGeofencing) {
	var featureCollection geojson.FeatureCollection
	featureCollection.UnmarshalJSON(feed.Data.GeofencingZones)
	for _, item := range featureCollection.Features {
		res, _ := geom.SetSRID(item.Geometry, 4326)
		obj := wkb.Geom{
			T: res,
		}

		q := dataProcessor.DB.QueryRow(
			`SELECT geom_hash
			FROM service_area
			WHERE geom_hash = ENCODE(DIGEST($1::bytea, 'sha1'), 'hex')`,
			&obj)
		var test string
		err := q.Scan(&test)
		if err == sql.ErrNoRows {
			dataProcessor.insertGeofence(obj, feed.OperatorID)
		}
		// if item.Properties["ride_start_allowed"].(bool) {
		// 	result.Features = append(result.Features, item)
		// }
	}
}

func (dataProcessor DataProcessor) insertGeofence(geometry wkb.Geom, operatorID string) {
	result, err := dataProcessor.DB.Query(
		`SELECT municipality
		FROM zones 
		WHERE zone_type = 'municipality'
		AND ST_intersects(ST_MakeValid(st_SetSRID(ST_GeomFromWKB($1::bytea),4326)), area);`,
		&geometry)
	if err != nil {
		log.Print(err)
	}
	var municipalities []string
	for result.Next() {
		var gmcode string
		result.Scan(&gmcode)
		municipalities = append(municipalities, gmcode)
	}

	_, err = dataProcessor.DB.Exec(
		`INSERT INTO service_area (geom_hash, geom, municipalities)
		VALUES (ENCODE(DIGEST($1::bytea, 'sha1'), 'hex'),
			ST_MakeValid(ST_SetSRID(ST_GeomFromWKB($2::bytea), 4326)), $3)
		returning geom_hash`,
		&geometry, &geometry, pq.Array(municipalities))
	log.Print(err)
}
