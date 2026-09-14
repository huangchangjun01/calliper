-- 000002_unify_prediction_labels.up.sql
-- Unify prediction label conventions:
--   period    short_term|medium_term|long_term  -> short|medium|long   (aligned with DB CHECK)
--   direction neutral                          -> flat                 (aligned with frontend/eval)
--   direction CHECK constraint updated to (up, down, flat).

-- Drop the old direction CHECK constraint so rows can be updated to 'flat'.
ALTER TABLE predictions DROP CONSTRAINT IF EXISTS predictions_direction_check;

-- Normalize legacy period values to the DB convention short|medium|long.
UPDATE predictions
SET period = CASE
    WHEN period IN ('short', 'short_term') THEN 'short'
    WHEN period IN ('medium', 'medium_term') THEN 'medium'
    WHEN period IN ('long', 'long_term') THEN 'long'
    ELSE 'short'
END;

-- Map the legacy 'neutral' direction to 'flat'.
UPDATE predictions
SET direction = 'flat'
WHERE direction = 'neutral';

-- Re-apply the direction CHECK constraint with the canonical value set.
ALTER TABLE predictions
    ADD CONSTRAINT predictions_direction_check
    CHECK (direction IN ('up', 'down', 'flat'));