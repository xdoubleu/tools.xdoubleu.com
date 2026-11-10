-- Routes by p75 CLS, split by device. A route needs >=10 samples and >=2 users.
SELECT properties.route AS route, properties.$device_type AS device_type,
  count() AS samples, uniq(person_id) AS users,
  quantile(0.75)(toFloat(properties.value)) AS p75_cls
FROM events
WHERE event = 'web_vital_cls' AND timestamp > now() - INTERVAL 14 DAY
GROUP BY route, device_type
HAVING samples >= 10 AND users >= 2
ORDER BY p75_cls DESC

-- Rage clicks within 5s after a mutation on the same route: join $rageclick
-- to the previous custom event of the session with lagInFrame, as in
-- mis-navigation.sql.
