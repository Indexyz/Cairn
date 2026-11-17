package domain

import (
	"strconv"
	"strings"
	"time"
)

// Cron is a parsed five-field cron expression: minute hour day-of-month month
// day-of-week.
type Cron struct {
	minute   [60]bool
	hour     [24]bool
	day      [32]bool
	month    [13]bool
	weekday  [8]bool
	dayStar  bool
	weekStar bool
}

// searchMinutes bounds Next; five years covers every valid expression.
const searchMinutes = 5 * 366 * 24 * 60

var cronMonths = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var cronWeekdays = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// ParseCron parses a five-field expression.
func ParseCron(expression string) (*Cron, error) {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return nil, fieldError("cron", "cron must have five fields: minute hour day month weekday", expression)
	}
	cron := &Cron{}
	if err := cron.fill(fields[0], 0, 59, nil, cron.minute[:]); err != nil {
		return nil, err
	}
	if err := cron.fill(fields[1], 0, 23, nil, cron.hour[:]); err != nil {
		return nil, err
	}
	if err := cron.fill(fields[2], 1, 31, nil, cron.day[:]); err != nil {
		return nil, err
	}
	if err := cron.fill(fields[3], 1, 12, cronMonths, cron.month[:]); err != nil {
		return nil, err
	}
	if err := cron.fill(fields[4], 0, 7, cronWeekdays, cron.weekday[:]); err != nil {
		return nil, err
	}
	cron.weekday[0] = cron.weekday[0] || cron.weekday[7]
	cron.dayStar = fields[2] == "*"
	cron.weekStar = fields[4] == "*"
	return cron, nil
}

func (cron *Cron) fill(field string, low, high int, names map[string]int, set []bool) error {
	invalid := fieldError("cron", "invalid cron field "+field, field)
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return invalid
		}
		step := 1
		if base, rawStep, found := strings.Cut(part, "/"); found {
			parsed, err := strconv.Atoi(rawStep)
			if err != nil || parsed <= 0 {
				return invalid
			}
			step = parsed
			part = base
		}
		start, end := low, high
		if part != "*" {
			rawStart, rawEnd, ranged := strings.Cut(part, "-")
			parsed, err := cronValue(rawStart, names)
			if err != nil {
				return invalid
			}
			start = parsed
			end = parsed
			if ranged {
				parsed, err = cronValue(rawEnd, names)
				if err != nil {
					return invalid
				}
				end = parsed
			}
		}
		if start < low || end > high || start > end {
			return invalid
		}
		for value := start; value <= end; value += step {
			set[value] = true
		}
	}
	return nil
}

func cronValue(raw string, names map[string]int) (int, error) {
	if value, known := names[strings.ToLower(raw)]; known {
		return value, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// Next returns the first occurrence strictly after `after`, in loc's wall clock.
func (cron *Cron) Next(after time.Time, loc *time.Location) (time.Time, error) {
	cursor := after.In(loc).Truncate(time.Minute).Add(time.Minute)
	for step := 0; step < searchMinutes; step++ {
		if cron.matches(cursor) {
			return cursor, nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return time.Time{}, fieldError("cron", "cron never fires", "")
}

func (cron *Cron) matches(instant time.Time) bool {
	if !cron.minute[instant.Minute()] || !cron.hour[instant.Hour()] || !cron.month[int(instant.Month())] {
		return false
	}
	dayMatches := cron.day[instant.Day()]
	weekMatches := cron.weekday[int(instant.Weekday())]
	switch {
	case cron.dayStar:
		return weekMatches
	case cron.weekStar:
		return dayMatches
	default:
		return dayMatches || weekMatches
	}
}

// ParseTimezone resolves an IANA zone name.
func ParseTimezone(name string) (*time.Location, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "UTC"
	}
	loc, err := time.LoadLocation(trimmed)
	if err != nil {
		return nil, fieldError("timezone", "unknown timezone "+trimmed, trimmed)
	}
	return loc, nil
}
