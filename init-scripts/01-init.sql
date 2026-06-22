CREATE SCHEMA IF NOT EXISTS auth_schema;
CREATE SCHEMA IF NOT EXISTS violation_schema;
CREATE SCHEMA IF NOT EXISTS rule_schema;

-- Auth Schema
CREATE TABLE auth_schema.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    hashedpass VARCHAR(255) NOT NULL,
    roles VARCHAR(50) NOT NULL -- 'OFFICER' or 'MEMBER'
);

CREATE TABLE auth_schema.user_vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth_schema.users(id),
    license_plate VARCHAR(50) NOT NULL,
    UNIQUE(user_id, license_plate)
);

-- Rule Schema
CREATE TABLE rule_schema.rule_sets (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    version INT UNIQUE NOT NULL,
    active_from TIMESTAMP WITH TIME ZONE NOT NULL,
    active_to TIMESTAMP WITH TIME ZONE,
    rules JSONB NOT NULL
);

-- Violation Schema
CREATE TABLE violation_schema.violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    license_plate VARCHAR(50) NOT NULL,
    violation_type VARCHAR(100) NOT NULL,
    location VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    photo_url TEXT,
    applied_rule_set_version INT, -- Logical reference to rule_schema.rule_sets version
    fine_amount DECIMAL(12, 2),
    status VARCHAR(50) NOT NULL DEFAULT 'UNPAID'
);

CREATE TABLE violation_schema.transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    license_plate VARCHAR(50) NOT NULL,
    violation_id UUID REFERENCES violation_schema.violations(id),
    amount DECIMAL(12, 2) NOT NULL,
    status VARCHAR(50) NOT NULL,
    external_tx_id VARCHAR(255)
);

-- Insert some default rule data for day one (Version 1)
INSERT INTO rule_schema.rule_sets (version, active_from, rules) VALUES (
    1,
    CURRENT_TIMESTAMP,
    '{
        "base_amount": {
            "expired_meter": 50000,
            "no_parking_zone": 150000,
            "blocking_hydrant": 250000,
            "disabled_spot": 500000
        },
        "time_multiplier": [
            {"start": "06:00", "end": "22:00", "multiplier": 1.0},
            {"start": "22:00", "end": "06:00", "multiplier": 1.5}
        ],
        "repeat_multiplier": [
            {"prior_unpaid": 0, "multiplier": 1.0},
            {"prior_unpaid": 1, "multiplier": 1.5},
            {"prior_unpaid": 2, "multiplier": 2.0}
        ]
    }'::jsonb
);

-- Insert a default officer and member for testing
-- Passwords should be hashed in code, but for local DB seeding we'll insert a dummy hash
INSERT INTO auth_schema.users (username, email, hashedpass, roles) VALUES 
('officer1', 'officer1@example.com', '$2a$10$alG/jyXvV4pwnjmk5FGp1uJ0W7AAE205bUiBMowcADRFGNIWPs6vC', 'OFFICER'),
('member1', 'member1@example.com', '$2a$10$alG/jyXvV4pwnjmk5FGp1uJ0W7AAE205bUiBMowcADRFGNIWPs6vC', 'MEMBER'),
('member2', 'member2@example.com', '$2a$10$alG/jyXvV4pwnjmk5FGp1uJ0W7AAE205bUiBMowcADRFGNIWPs6vC', 'MEMBER');

INSERT INTO auth_schema.user_vehicles (user_id, license_plate)
SELECT id, 'MOCK123' FROM auth_schema.users WHERE username = 'member1';

INSERT INTO auth_schema.user_vehicles (user_id, license_plate)
SELECT id, 'B1234XYZ' FROM auth_schema.users WHERE username = 'member1';

INSERT INTO auth_schema.user_vehicles (user_id, license_plate)
SELECT id, 'D5678ABC' FROM auth_schema.users WHERE username = 'member2';

INSERT INTO auth_schema.user_vehicles (user_id, license_plate)
SELECT id, 'F9012DEF' FROM auth_schema.users WHERE username = 'member2';
