CREATE TABLE IF NOT EXISTS indicators (
    id SMALLINT PRIMARY KEY,
    text_id VARCHAR NOT NULL UNIQUE,
    description TEXT NOT NULL,
    first_day DATE NOT NULL DEFAULT '2019-12-31',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_indicators_text_id
ON indicators (text_id);
