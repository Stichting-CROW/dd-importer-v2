CREATE INDEX idx_day_statistics_geometry_date
ON public.day_statistics
  (geometry_ref, date, indicator, system_id, vehicle_type)
INCLUDE (value);

CREATE INDEX idx_day_statistics_system_date
ON public.day_statistics
  (system_id, date, indicator, geometry_ref, vehicle_type)
INCLUDE (value);

DROP INDEX IF EXISTS idx_day_statistics_covering;
