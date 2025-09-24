-- Add last_fetched_at column to feeds table
ALTER TABLE feeds
ADD COLUMN last_fetched_at TIMESTAMP WITH TIME ZONE NULL;