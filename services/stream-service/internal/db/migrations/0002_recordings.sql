-- NULL until a creator uploads a recording for this stream (see
-- internal/storage and POST /streams/{id}/recording). No separate
-- recordings table since it's a strict 1:1 with a stream for now; revisit
-- if streams ever get multiple recording qualities/segments.
ALTER TABLE streams ADD COLUMN recording_url TEXT;
