package auth

import "time"

const fiveMinutes = 5 * time.Minute

func testTime() time.Time {
	return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
}
