-- Searches are visible to every signed-in user, so the grid is now ordered
-- globally by (created_at, id) rather than per user_email. The existing
-- searches_user_created_idx (user_email, created_at DESC) still covers the
-- filter-by-email case: it bounds the scan by created_at, leaving only the
-- id tiebreak as a filter.
CREATE INDEX searches_created_idx ON searches (created_at DESC, id DESC);
