-- migrations/0005_jobs_retry_at.sql
-- Nouvelle tentative différée : un job échoué repasse 'queued' avec un
-- retry_at futur (backoff exponentiel 2^attempts × 30 s, M4). Le dequeue
-- ignore les jobs dont retry_at n'est pas encore atteint.
-- +goose Up
ALTER TABLE jobs ADD COLUMN retry_at DATETIME;

-- +goose Down
ALTER TABLE jobs DROP COLUMN retry_at;
