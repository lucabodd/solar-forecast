package domain

import (
	"math"
	"time"
)

// CalculateSunriseSunset calculates sunrise and sunset times for a given date and location.
// Uses the NOAA solar position algorithm.
// Returns sunrise and sunset times in the location's timezone.
func CalculateSunriseSunset(date time.Time, latitude, longitude float64) (sunrise, sunset time.Time) {
	// Get the location from the input date
	loc := date.Location()

	// Calculate Julian day at noon
	year, month, day := date.Date()
	jd := julianDay(year, int(month), day)

	// Calculate solar noon and day length
	sunriseJD, sunsetJD := calculateSunTimes(jd, latitude, longitude)

	// Convert Julian day fractions to time
	sunrise = julianDayToTime(sunriseJD, loc)
	sunset = julianDayToTime(sunsetJD, loc)

	return sunrise, sunset
}

// julianDay calculates the Julian day number for a given date at noon UT
func julianDay(year, month, day int) float64 {
	if month <= 2 {
		year--
		month += 12
	}

	a := year / 100
	b := 2 - a + a/4

	return float64(int(365.25*float64(year+4716))) +
		float64(int(30.6001*float64(month+1))) +
		float64(day) + float64(b) - 1524.5
}

// calculateSunTimes calculates sunrise and sunset Julian day values
func calculateSunTimes(jd, latitude, longitude float64) (sunriseJD, sunsetJD float64) {
	// Julian century
	t := (jd - 2451545.0) / 36525.0

	// Geometric mean longitude of the sun (degrees)
	l0 := math.Mod(280.46646+t*(36000.76983+0.0003032*t), 360)

	// Geometric mean anomaly of the sun (degrees)
	m := 357.52911 + t*(35999.05029-0.0001537*t)

	// Eccentricity of Earth's orbit
	e := 0.016708634 - t*(0.000042037+0.0000001267*t)

	// Equation of center
	mRad := m * math.Pi / 180
	c := (1.914602 - t*(0.004817+0.000014*t)) * math.Sin(mRad)
	c += (0.019993 - 0.000101*t) * math.Sin(2*mRad)
	c += 0.000289 * math.Sin(3*mRad)

	// Sun's true longitude
	sunLon := l0 + c

	// Sun's apparent longitude
	omega := 125.04 - 1934.136*t
	sunAppLon := sunLon - 0.00569 - 0.00478*math.Sin(omega*math.Pi/180)

	// Mean obliquity of the ecliptic (degrees)
	obliq := 23.0 + (26.0+(21.448-t*(46.8150+t*(0.00059-t*0.001813)))/60.0)/60.0

	// Corrected obliquity
	obliqCorr := obliq + 0.00256*math.Cos(omega*math.Pi/180)

	// Sun's declination
	sunDeclin := math.Asin(math.Sin(obliqCorr*math.Pi/180) * math.Sin(sunAppLon*math.Pi/180))

	// Equation of time (minutes)
	y := math.Tan(obliqCorr * math.Pi / 360)
	y = y * y
	l0Rad := l0 * math.Pi / 180
	eqTime := 4 * (y*math.Sin(2*l0Rad) -
		2*e*math.Sin(mRad) +
		4*e*y*math.Sin(mRad)*math.Cos(2*l0Rad) -
		0.5*y*y*math.Sin(4*l0Rad) -
		1.25*e*e*math.Sin(2*mRad)) * 180 / math.Pi

	// Hour angle at sunrise/sunset
	// Uses standard refraction of 0.833 degrees (50 arcminutes)
	latRad := latitude * math.Pi / 180
	zenith := 90.833 * math.Pi / 180 // Standard refraction

	cosHA := (math.Cos(zenith) / (math.Cos(latRad) * math.Cos(sunDeclin))) -
		math.Tan(latRad)*math.Tan(sunDeclin)

	// Check for polar day/night
	if cosHA > 1 {
		// Sun never rises (polar night)
		// Return noon as both sunrise and sunset
		noon := jd + (720-longitude*4-eqTime)/1440
		return noon, noon
	}
	if cosHA < -1 {
		// Sun never sets (polar day)
		// Return midnight and next midnight
		return jd - 0.5, jd + 0.5
	}

	ha := math.Acos(cosHA) * 180 / math.Pi

	// Solar noon (in minutes from midnight UTC)
	solarNoon := 720 - longitude*4 - eqTime

	// Sunrise and sunset times (in minutes from midnight UTC)
	sunriseMinutes := solarNoon - ha*4
	sunsetMinutes := solarNoon + ha*4

	// Convert to Julian day
	sunriseJD = jd + sunriseMinutes/1440
	sunsetJD = jd + sunsetMinutes/1440

	return sunriseJD, sunsetJD
}

// julianDayToTime converts a Julian day to a time.Time in the given location
func julianDayToTime(jd float64, loc *time.Location) time.Time {
	// Convert Julian day to Unix timestamp
	// Julian day 2440587.5 = Unix epoch (Jan 1, 1970 00:00:00 UTC)
	unixSeconds := (jd - 2440587.5) * 86400

	t := time.Unix(int64(unixSeconds), int64((unixSeconds-float64(int64(unixSeconds)))*1e9))
	return t.In(loc)
}

// IsDaylight returns true if the given time is between sunrise and sunset
func IsDaylight(t time.Time, latitude, longitude float64) bool {
	sunrise, sunset := CalculateSunriseSunset(t, latitude, longitude)
	return t.After(sunrise) && t.Before(sunset)
}
