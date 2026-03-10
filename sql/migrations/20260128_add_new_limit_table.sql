CREATE TABLE geometry_operator_modality_limit (
    geometry_operator_modality_limit_id SERIAL PRIMARY KEY,

    geometry_ref VARCHAR(255) NOT NULL,
    operator VARCHAR(255) NOT NULL,
    form_factor VARCHAR(255) NOT NULL,
    propulsion_type VARCHAR(255) NOT NULL,
    effective_date DATE NOT NULL,

    limits JSONB NOT NULL,

    UNIQUE (
        geometry_ref,
        operator,
        form_factor,
        propulsion_type
        effective_date
    )
);
CREATE INDEX idx_geometry_operator_modality_limit_effective_date
        ON geometry_operator_modality_limit (geometry_ref, operator, form_factor, propulsion_type, effective_date);


                    WHEN 1 THEN 'vehicle_cap'
                    WHEN 6 THEN 'number_of_wrongly_parked_vehicles'

INSERT INTO geometry_operator_modality_limit (
    geometry_ref,
    operator,
    form_factor,
    propulsion_type,
    effective_date,
    limits
) VALUES
    (
        'cbs:GM0599',
        'check'
        'moped',
        'electric',
        '2025-01-01',
        '{
        "percentage_parked_longer_then_24_hours": 20, 
        "percentage_parked_longer_then_3_days": 10, 
        "percentage_parked_longer_then_7_days": 5, 
        "percentage_parked_longer_then_14_days": 2,
        "vehicle_cap": 500,
        "number_of_wrongly_parked_vehicles": 20
        }'
    );

INSERT INTO geometry_operator_modality_limit (
    geometry_ref,
    operator,
    form_factor,
    propulsion_type,
    effective_date,
    limits
) VALUES
    (
        'cbs:GM0599',
        'check'
        'moped',
        'electric',
        '2025-12-01',
        '{
            "percentage_parked_longer_then_24_hours": 70, 
            "percentage_parked_longer_then_3_days": 60, 
            "percentage_parked_longer_then_7_days": 50, 
            "percentage_parked_longer_then_14_days": 10,
            "vehicle_cap": 500,
            "number_of_wrongly_parked_vehicles": 20
        }'
    );
