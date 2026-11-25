# Feed validator schemas

Official JSON Schemas used by the feed validator (`cmd/feed_validator`). The files are copied
unchanged, except MDS 2.0, which is generated.

| Directory | Source | Pinned at | License |
| --- | --- | --- | --- |
| `gbfs/<version>/` | [MobilityData/gbfs-json-schema](https://github.com/MobilityData/gbfs-json-schema), folder `v<version>` | commit `bae2d21` | Apache 2.0 (`gbfs/LICENSE`) |
| `mds/1.0/`, `mds/1.1/`, `mds/1.2/` | [openmobilityfoundation/mobility-data-specification](https://github.com/openmobilityfoundation/mobility-data-specification), tags `1.0.0`, `1.1.0`, `1.2.0`, folder `provider` | tags | CC BY 4.0 (`mds/LICENSE`) |
| `mds/2.0/` | [openmobilityfoundation/mds-openapi](https://github.com/openmobilityfoundation/mds-openapi), `reference/provider.yaml` | commit `0c07bc3` | CC BY 4.0 (`mds/LICENSE`) |

Only the files the importer reads are included: `gbfs.json`, `free_bike_status.json`
(1.x and 2.x), `vehicle_status.json` (3.x), `vehicle_types.json`, `geofencing_zones.json`,
`manifest.json`, and the MDS provider `vehicles`, `vehicles/status` and `trips` responses.

## Regenerating MDS 2.0

MDS 2.0 is published as a split OpenAPI 3.1 document. `bundle_mds2.py` inlines all
references of the `200` responses of `/vehicles`, `/vehicles/status` and `/trips`:

```bash
git clone https://github.com/openmobilityfoundation/mds-openapi /tmp/mds-openapi
python3 validator/schemas/bundle_mds2.py /tmp/mds-openapi validator/schemas/mds/2.0
```

## Known upstream issues

- The MDS `1.1.0` schemas still require `version` to match `1.0.x`. The validator ignores
  `/version` errors when the declared version picked the schema (see `validator/mds.go`).
- The MDS 2.0 mode event models write `contains: [values]`, which is not valid JSON Schema.
  `bundle_mds2.py` rewrites it to `contains: {enum: [values]}`.
- MDS 2.0 combines the mode models (car-share, delivery-robots, micromobility,
  passenger-services) with `oneOf`, but they overlap, so a plain scooter matches several
  modes and fails. `bundle_mds2.py` uses `anyOf` for these: a record must fit at least one mode.
