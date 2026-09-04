package mocks

import (
	"archive/zip"
	"bytes"
)

// SampleFeedFiles returns a minimal GTFS static file set that exercises
// every trap from issue #1390:
//   - calendar.txt has all-zero weekday flags (the decoy)
//   - calendar_dates.txt is additions only (exception_type=1) and is the
//     sole source of operating days
//   - stop_times.txt carries an absurd 87:39:00 value that must be rejected
//   - one stop_times row is a non-boarding technical pass-through
//   - columns are alphabetically ordered, as in the real feed
//   - transfers.txt (issue #1391) carries one valid row plus one with a
//     blank from_stop_id, exercising parseTransfers' skip branch
func SampleFeedFiles() map[string]string {
	return map[string]string{
		"feed_info.txt": "feed_end_date,feed_lang,feed_publisher_name," +
			"feed_start_date,feed_version\n" +
			"20261212,fr,NMBS-SNCB,20260630,2026-08-31\n",
		"stops.txt": "location_type,parent_station,platform_code,stop_id," +
			"stop_lat,stop_lon,stop_name\n" +
			"1,,,gs:nmbssncb:S8814001,50.83,4.33,Bruxelles-Midi\n" +
			"0,gs:nmbssncb:S8814001,3,gs:nmbssncb:8814001_3,50.83,4.33,Bruxelles-Midi\n" +
			"1,,,gs:nmbssncb:S8892007,51.03,3.71,Gent-Sint-Pieters\n" +
			"0,gs:nmbssncb:S8892007,1,gs:nmbssncb:8892007_1,51.03,3.71,Gent-Sint-Pieters\n" +
			"0,,,gs:nmbssncb:8821006,51.22,4.42,Antwerpen-Centraal\n",
		"routes.txt": "route_id,route_long_name,route_short_name,route_type\n" +
			"r1,Brussels - Ghent,IC,2\n",
		"trips.txt": "direction_id,route_id,service_id,trip_headsign," +
			"trip_id,trip_short_name\n" +
			"0,r1,svc_weekday,Gent-Sint-Pieters,trip_522_a,522\n" +
			"0,r1,svc_weekday,Gent-Sint-Pieters,trip_522_b,522\n",
		"calendar.txt": "end_date,friday,monday,saturday,service_id," +
			"start_date,sunday,thursday,tuesday,wednesday\n" +
			"20261212,0,0,0,svc_weekday,20260630,0,0,0,0\n",
		"calendar_dates.txt": "date,exception_type,service_id\n" +
			"20261001,1,svc_weekday\n" +
			"20261002,1,svc_weekday\n" +
			"20261003,1,svc_weekday\n",
		"stop_times.txt": "arrival_time,departure_time,drop_off_type," +
			"pickup_type,stop_id,stop_sequence,trip_id\n" +
			"08:00:00,08:00:00,0,0,gs:nmbssncb:8814001_3,1,trip_522_a\n" +
			"08:20:00,08:20:00,1,1,gs:nmbssncb:8821006,2,trip_522_a\n" +
			"08:40:00,08:40:00,0,0,gs:nmbssncb:8892007_1,3,trip_522_a\n" +
			"87:39:00,87:39:00,0,0,gs:nmbssncb:8892007_1,1,trip_522_b\n",
		"transfers.txt": "from_stop_id,min_transfer_time,to_stop_id,transfer_type\n" +
			"gs:nmbssncb:8814001_3,120,gs:nmbssncb:8892007_1,2\n" +
			",120,gs:nmbssncb:8892007_1,2\n",
	}
}

// SampleServiceDate is a date every trip in SampleFeedFiles resolves to,
// via calendar_dates alone.
const SampleServiceDate = "2026-10-01"

// BuildFeedZip packs files into an in-memory zip (valid PK\x03\x04 header).
func BuildFeedZip(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	return buf.Bytes()
}
