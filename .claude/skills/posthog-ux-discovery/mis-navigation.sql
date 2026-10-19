-- List -> detail exits within 8s. Fill in the two path placeholders.
-- HogQL rejects QUALIFY and has no nextIf: use lagInFrame/leadInFrame.
SELECT prev_path, path, count() AS exits FROM (
  SELECT properties.$pathname AS path, timestamp,
    lagInFrame(properties.$pathname) OVER w AS prev_path,
    leadInFrame(timestamp) OVER w AS next_ts
  FROM events
  WHERE event = '$pageview' AND timestamp > now() - INTERVAL 14 DAY
  WINDOW w AS (PARTITION BY $session_id ORDER BY timestamp
    ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING)
)
WHERE prev_path = '<list path>' AND path LIKE '<detail prefix>%'
  AND next_ts IS NOT NULL AND dateDiff('second', timestamp, next_ts) < 8
GROUP BY prev_path, path ORDER BY exits DESC
