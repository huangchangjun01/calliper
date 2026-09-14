-- 000002_unify_prediction_labels.down.sql
-- Rollback of the prediction label unification migration.

-- Drop the canonical (up, down, flat) constraint.
ALTER TABLE predictions DROP CONSTRAINT IF EXISTS predictions_direction_check;

-- Restore legacy normalized period values (reversible mapping).
UPDATE predictions
SET period = CASE
    WHEN period = 'short' THEN 'short_term'
    WHEN period = 'medium' THEN 'medium_term'
    WHEN period = 'long' THEN 'long_term'
    ELSE 'short_term'
END;

-- Map the canonical 'flat' back to the legacy 'neutral' direction.
UPDATE predictions
SET direction = 'neutral'
WHERE direction = 'flat';

-- Restore the original direction CHECK constraint.
ALTER TABLE predictions
    ADD CONSTRAINT predictions_direction_check
    CHECK (direction IN ('up', 'down', 'neutral'));