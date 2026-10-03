-- Day 6–8: institutions calendar, routes, pricing, subscriptions.
-- The tables exist since 0001; this migration seeds reference data and adds
-- the indexes the new queries need.

-- +goose Up
CREATE INDEX ix_route_shifts_route ON route_shifts (route_id);
CREATE INDEX ix_route_stops_stop ON route_stops (stop_id);
CREATE INDEX ix_seats_subscription ON seat_assignments (subscription_id);
CREATE INDEX ix_seats_rider ON seat_assignments (rider_id) WHERE status <> 'RELEASED';
CREATE INDEX ix_calendar_exceptions_date ON calendar_exceptions (service_date);
CREATE INDEX ix_absences_date ON absences (service_date);
CREATE INDEX ix_stops_name ON stops (lower(name));

-- Plans (design doc 5.2): weekly 0%, monthly 5%, term 10%.
INSERT INTO plans (id, code, name, period, discount_bps) VALUES
  ('01a0fa00-0000-7000-8000-000000000001', 'WEEKLY',  'Weekly',  'WEEKLY',  0),
  ('01a0fa00-0000-7000-8000-000000000002', 'MONTHLY', 'Monthly', 'MONTHLY', 500),
  ('01a0fa00-0000-7000-8000-000000000003', 'TERM',    'Term',    'TERM',    1000)
ON CONFLICT (code) DO NOTHING;

-- Starting pricing policies for Addis Ababa. Minibus and sedan use the
-- design doc's illustrative figures (section 5.2); midibus and SUV are
-- extrapolated. Replace them through the pricing admin API before launch.
INSERT INTO pricing_policies (id, city, vehicle_class, base_fee_cents, per_km_cents, per_min_cents,
                              commission_bps, min_occupancy, valid) VALUES
  ('01a0fb00-0000-7000-8000-000000000001', 'Addis Ababa', 'MINIBUS', 15000, 3000, 300, 1500, 10, '[2026-01-01,)'),
  ('01a0fb00-0000-7000-8000-000000000002', 'Addis Ababa', 'MIDIBUS', 17000, 3200, 320, 1500, 12, '[2026-01-01,)'),
  ('01a0fb00-0000-7000-8000-000000000003', 'Addis Ababa', 'SEDAN',    8000, 1400, 150, 1500,  3, '[2026-01-01,)'),
  ('01a0fb00-0000-7000-8000-000000000004', 'Addis Ababa', 'SUV',      9000, 1600, 170, 1500,  4, '[2026-01-01,)')
ON CONFLICT (id) DO NOTHING;

-- Ethiopian public holidays, city-wide (institution_id NULL), FR-RTE-5.
-- Eid and Mawlid follow the lunar calendar: the dates below are estimates
-- and must be confirmed when the government announces them.
INSERT INTO calendar_exceptions (id, institution_id, service_date, kind, note) VALUES
  (gen_random_uuid(), NULL, '2026-01-07', 'HOLIDAY', 'Ethiopian Christmas (Genna)'),
  (gen_random_uuid(), NULL, '2026-01-19', 'HOLIDAY', 'Epiphany (Timkat)'),
  (gen_random_uuid(), NULL, '2026-03-02', 'HOLIDAY', 'Victory of Adwa'),
  (gen_random_uuid(), NULL, '2026-03-20', 'HOLIDAY', 'Eid al-Fitr (estimated)'),
  (gen_random_uuid(), NULL, '2026-04-10', 'HOLIDAY', 'Ethiopian Good Friday (Siklet)'),
  (gen_random_uuid(), NULL, '2026-04-12', 'HOLIDAY', 'Ethiopian Easter (Fasika)'),
  (gen_random_uuid(), NULL, '2026-05-01', 'HOLIDAY', 'International Workers'' Day'),
  (gen_random_uuid(), NULL, '2026-05-05', 'HOLIDAY', 'Patriots'' Victory Day'),
  (gen_random_uuid(), NULL, '2026-05-27', 'HOLIDAY', 'Eid al-Adha (estimated)'),
  (gen_random_uuid(), NULL, '2026-05-28', 'HOLIDAY', 'Downfall of the Derg'),
  (gen_random_uuid(), NULL, '2026-08-26', 'HOLIDAY', 'Mawlid (estimated)'),
  (gen_random_uuid(), NULL, '2026-09-11', 'HOLIDAY', 'Ethiopian New Year (Enkutatash)'),
  (gen_random_uuid(), NULL, '2026-09-27', 'HOLIDAY', 'Finding of the True Cross (Meskel)'),
  (gen_random_uuid(), NULL, '2027-01-07', 'HOLIDAY', 'Ethiopian Christmas (Genna)'),
  (gen_random_uuid(), NULL, '2027-01-19', 'HOLIDAY', 'Epiphany (Timkat)'),
  (gen_random_uuid(), NULL, '2027-03-02', 'HOLIDAY', 'Victory of Adwa'),
  (gen_random_uuid(), NULL, '2027-03-10', 'HOLIDAY', 'Eid al-Fitr (estimated)'),
  (gen_random_uuid(), NULL, '2027-04-30', 'HOLIDAY', 'Ethiopian Good Friday (Siklet)'),
  (gen_random_uuid(), NULL, '2027-05-01', 'HOLIDAY', 'International Workers'' Day'),
  (gen_random_uuid(), NULL, '2027-05-02', 'HOLIDAY', 'Ethiopian Easter (Fasika)'),
  (gen_random_uuid(), NULL, '2027-05-05', 'HOLIDAY', 'Patriots'' Victory Day'),
  (gen_random_uuid(), NULL, '2027-05-17', 'HOLIDAY', 'Eid al-Adha (estimated)'),
  (gen_random_uuid(), NULL, '2027-05-28', 'HOLIDAY', 'Downfall of the Derg'),
  (gen_random_uuid(), NULL, '2027-08-15', 'HOLIDAY', 'Mawlid (estimated)'),
  (gen_random_uuid(), NULL, '2027-09-12', 'HOLIDAY', 'Ethiopian New Year (Enkutatash)'),
  (gen_random_uuid(), NULL, '2027-09-28', 'HOLIDAY', 'Finding of the True Cross (Meskel)')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM calendar_exceptions WHERE institution_id IS NULL AND kind = 'HOLIDAY'
  AND service_date BETWEEN '2026-01-01' AND '2027-12-31';
DELETE FROM pricing_policies WHERE id IN ('01a0fb00-0000-7000-8000-000000000001', '01a0fb00-0000-7000-8000-000000000002',
                                          '01a0fb00-0000-7000-8000-000000000003', '01a0fb00-0000-7000-8000-000000000004');
DELETE FROM plans WHERE code IN ('WEEKLY', 'MONTHLY', 'TERM')
  AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.plan_id = plans.id);
DROP INDEX ix_stops_name;
DROP INDEX ix_absences_date;
DROP INDEX ix_calendar_exceptions_date;
DROP INDEX ix_seats_rider;
DROP INDEX ix_seats_subscription;
DROP INDEX ix_route_stops_stop;
DROP INDEX ix_route_shifts_route;
