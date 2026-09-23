-- Export new MDS trips feeds derived from active MDS vehicle feeds to CSV.
--
-- The CSV is written to stdout, so it works from a Docker container with local
-- file access. Server-side COPY TO STDOUT is used because it supports
-- multi-line queries under `psql -f` (unlike the \copy meta-command).
--
-- Local psql:
--   psql "$DATABASE_URL" -f sql/export_mds_trips_feeds.sql > sql/mds_trips_feeds_new.csv
--
-- Docker (run the script inside the container, capture stdout on the host):
--   docker exec -i <container> psql "$DATABASE_URL" -f sql/export_mds_trips_feeds.sql > sql/mds_trips_feeds_new.csv
COPY (
    SELECT
        f.system_id,
        regexp_replace(f.feed_url, '/vehicles$', '/trips') AS feed_url,
        CASE f.feed_type WHEN 'mds' THEN 'mds-trips-v1'
                         WHEN 'mds-v2' THEN 'mds-trips-v2' END AS feed_type,
        f.import_strategy,
        f.authentication,
        f.request_headers,
        f.default_vehicle_type,
        TRUE  AS is_active,
        FALSE AS import_vehicles,
        FALSE AS import_service_area,
        'Created from feed ' || f.feed_id || ' (' || f.feed_type || ') for MDS trips import' AS remarks
    FROM feeds f
    WHERE f.feed_type IN ('mds', 'mds-v2')
      AND f.is_active = TRUE
      AND f.feed_url LIKE '%/vehicles'
      AND NOT EXISTS (
          SELECT 1 FROM feeds d
          WHERE d.system_id = f.system_id
            AND d.feed_url = regexp_replace(f.feed_url, '/vehicles$', '/trips')
            AND d.feed_type = CASE f.feed_type WHEN 'mds' THEN 'mds-trips-v1'
                                               WHEN 'mds-v2' THEN 'mds-trips-v2' END
            AND d.authentication::jsonb IS NOT DISTINCT FROM f.authentication::jsonb
            AND d.request_headers::jsonb IS NOT DISTINCT FROM f.request_headers::jsonb
      )
    ORDER BY f.system_id, f.feed_id
) TO STDOUT WITH (FORMAT CSV, HEADER);
