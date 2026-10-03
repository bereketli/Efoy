-- +goose Up
-- Activation and suspension (FR-DRV-3, FR-DRV-5). A suspension is either
-- automatic (a required document expired or is missing; lifted automatically
-- when it is renewed) or by an admin (lifted only by an admin).
ALTER TABLE drivers
  ADD COLUMN suspension_source text CHECK (suspension_source IN ('DOCUMENTS','ADMIN'));
ALTER TABLE vehicles
  ADD COLUMN suspended_reason  text,
  ADD COLUMN suspension_source text CHECK (suspension_source IN ('DOCUMENTS','ADMIN'));
ALTER TABLE fleet_owners
  ADD COLUMN suspended_reason text;

CREATE INDEX ix_dva_driver ON driver_vehicle_assignments (driver_id);
CREATE INDEX ix_vehicles_fleet ON vehicles (fleet_owner_id) WHERE fleet_owner_id IS NOT NULL;

-- Owner-drivers drive their own vehicle: give vehicles registered before
-- assignments existed their owner as primary driver.
INSERT INTO driver_vehicle_assignments (id, driver_id, vehicle_id, valid, is_primary)
SELECT gen_random_uuid(), v.owner_driver_id, v.id, tstzrange(v.created_at, NULL), true
  FROM vehicles v
 WHERE v.owner_driver_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM driver_vehicle_assignments a WHERE a.vehicle_id = v.id);

-- Operating zones for standby and dispatch (design doc 6.1). Approximate
-- rectangles that partition the city without overlapping; refine them with
-- operations before launch.
INSERT INTO zones (id, city, name, boundary, min_standby) VALUES
  (gen_random_uuid(), 'Addis Ababa', 'Arada',
   ST_GeogFromText('SRID=4326;MULTIPOLYGON(((38.70 9.03, 38.80 9.03, 38.80 9.10, 38.70 9.10, 38.70 9.03)))'), 2),
  (gen_random_uuid(), 'Addis Ababa', 'Yeka',
   ST_GeogFromText('SRID=4326;MULTIPOLYGON(((38.80 9.00, 38.92 9.00, 38.92 9.10, 38.80 9.10, 38.80 9.00)))'), 2),
  (gen_random_uuid(), 'Addis Ababa', 'Kolfe',
   ST_GeogFromText('SRID=4326;MULTIPOLYGON(((38.66 8.93, 38.74 8.93, 38.74 9.03, 38.70 9.03, 38.70 9.10, 38.66 9.10, 38.66 8.93)))'), 2),
  (gen_random_uuid(), 'Addis Ababa', 'Kirkos',
   ST_GeogFromText('SRID=4326;MULTIPOLYGON(((38.66 8.86, 38.80 8.86, 38.80 9.03, 38.74 9.03, 38.74 8.93, 38.66 8.93, 38.66 8.86)))'), 2),
  (gen_random_uuid(), 'Addis Ababa', 'Bole',
   ST_GeogFromText('SRID=4326;MULTIPOLYGON(((38.80 8.86, 38.92 8.86, 38.92 9.00, 38.80 9.00, 38.80 8.86)))'), 2)
ON CONFLICT (city, name) DO NOTHING;

-- +goose Down
DELETE FROM zones WHERE city = 'Addis Ababa' AND name IN ('Arada','Yeka','Kolfe','Kirkos','Bole')
  AND NOT EXISTS (SELECT 1 FROM standby_shifts s WHERE s.zone_id = zones.id);
DROP INDEX ix_vehicles_fleet;
DROP INDEX ix_dva_driver;
ALTER TABLE fleet_owners DROP COLUMN suspended_reason;
ALTER TABLE vehicles DROP COLUMN suspension_source, DROP COLUMN suspended_reason;
ALTER TABLE drivers DROP COLUMN suspension_source;
