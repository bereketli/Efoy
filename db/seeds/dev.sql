-- Development data only (make seed-dev). Never run against staging or prod.
-- Two institutions so guardians can add children before institution
-- management lands (day 6).
INSERT INTO institutions (id, name, name_am, type, address, location, campus, contact_name, contact_phone, status)
VALUES
  ('01a0f900-0000-7000-8000-000000000001', 'Kazanchis Academy (demo)', 'ቃዛንቺስ አካዳሚ (ሙከራ)', 'SCHOOL',
   'Kazanchis, Kirkos, Addis Ababa', ST_GeogFromText('SRID=4326;POINT(38.7660 9.0170)'),
   ST_GeogFromText('SRID=4326;POLYGON((38.7650 9.0162, 38.7672 9.0162, 38.7672 9.0178, 38.7650 9.0178, 38.7650 9.0162))'),
   'Front office', '+251111000001', 'ACTIVE'),
  ('01a0f900-0000-7000-8000-000000000002', 'Ministry of Transport (demo)', 'የትራንስፖርት ሚኒስቴር (ሙከራ)', 'GOVERNMENT_OFFICE',
   'Mexico Square, Kirkos, Addis Ababa', ST_GeogFromText('SRID=4326;POINT(38.7457 9.0105)'),
   NULL, 'HR office', '+251111000002', 'ACTIVE')
ON CONFLICT (id) DO NOTHING;
