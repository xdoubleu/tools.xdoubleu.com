-- List -> detail exits within 8s. Fill in the two path placeholders.
-- HogQL rejects QUALIFY and has no nextIf: use lagInFrame/leadInFrame.
-- Window outputs are only visible to the outer query if projected in the
-- inner SELECT, so person_id and device_type are projected there to give
-- users (the >=2 users threshold) per device_type.
SELECT prev_path, path, device_type, count() AS exits, uniq(person_id) AS users FROM (
  SELECT properties.$pathname AS path, timestamp, person_id,
    properties.$device_type AS device_type,
    lagInFrame(properties.$pathname) OVER w AS prev_path,
    leadInFrame(timestamp) OVER w AS next_ts
  FROM events
  WHERE event = '$pageview' AND timestamp > now() - INTERVAL 14 DAY
  WINDOW w AS (PARTITION BY $session_id ORDER BY timestamp
    ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING)
)
WHERE prev_path = '<list path>' AND path LIKE '<detail prefix>%'
  AND next_ts IS NOT NULL AND dateDiff('second', timestamp, next_ts) < 8
GROUP BY prev_path, path, device_type ORDER BY exits DESC
