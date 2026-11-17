package domain

import (
	"testing"
	"time"
)

func TestParseCronRejectsBadExpressions(t *testing.T) {
	for _, expression := range []string{
		"* * * *", "* * * * * *", "*/0 * * * *", "70 * * * *", "a * * * *",
		"5-1 * * * *", "0 0 0 * *", "0 0 * 13 *", "0 0 * * 8", "1,,2 * * * *",
	} {
		if _, err := ParseCron(expression); err == nil {
			t.Fatalf("%q parsed but should not", expression)
		}
	}
}

func TestCronNext(t *testing.T) {
	zone := time.FixedZone("UTC+2", 2*60*60)
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, zone)
	cases := []struct {
		expression string
		after      time.Time
		want       time.Time
	}{
		{"*/15 * * * *", base.Add(10*time.Hour + 7*time.Minute), base.Add(10*time.Hour + 15*time.Minute)},
		{"0 9 * * 1-5", time.Date(2026, 3, 6, 10, 0, 0, 0, zone), time.Date(2026, 3, 9, 9, 0, 0, 0, zone)},
		{"0 0 1 JAN *", time.Date(2026, 3, 1, 0, 0, 0, 0, zone), time.Date(2027, 1, 1, 0, 0, 0, 0, zone)},
		// day-of-month and day-of-week are OR-ed when both are restricted
		{"0 0 13 * FRI", time.Date(2026, 3, 1, 0, 0, 0, 0, zone), time.Date(2026, 3, 6, 0, 0, 0, 0, zone)},
		{"30 4 * * *", base.Add(4*time.Hour + 10*time.Minute), base.Add(4*time.Hour + 30*time.Minute)},
		{"5 0 * * SUN", time.Date(2026, 3, 1, 0, 6, 0, 0, zone), time.Date(2026, 3, 8, 0, 5, 0, 0, zone)},
	}
	for _, testCase := range cases {
		cron, err := ParseCron(testCase.expression)
		if err != nil {
			t.Fatalf("parse %q: %v", testCase.expression, err)
		}
		next, err := cron.Next(testCase.after, zone)
		if err != nil {
			t.Fatalf("next for %q: %v", testCase.expression, err)
		}
		if !next.Equal(testCase.want) {
			t.Fatalf("%q: got %s, want %s", testCase.expression, next, testCase.want)
		}
	}
}

func TestCronNextNeverFires(t *testing.T) {
	cron, err := ParseCron("0 0 31 2 *")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := cron.Next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.UTC); err == nil {
		t.Fatal("expected an error for february 31st")
	}
}

func TestScheduleNextRunRendersUTC(t *testing.T) {
	schedule := &Schedule{Cron: "0 9 * * *", Timezone: "Asia/Shanghai"}
	next, err := schedule.NextRun(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("next run: %v", err)
	}
	if next != "2026-03-01T01:00:00Z" {
		t.Fatalf("got %s", next)
	}
}

func TestParseTimezone(t *testing.T) {
	if loc, err := ParseTimezone(""); err != nil || loc != time.UTC {
		t.Fatalf("empty timezone should default to UTC, got %v %v", loc, err)
	}
	if _, err := ParseTimezone("Mars/Base"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseScheduleValidates(t *testing.T) {
	cases := map[string]string{
		`{"name":"n","cron":"* * * *","timezone":"UTC","title":"t","origin":"o","goal":"g"}`:                 "cron",
		`{"name":"n","cron":"* * * * *","timezone":"Mars/Base","title":"t","origin":"o","goal":"g"}`:         "timezone",
		`{"name":"n","cron":"* * * * *","timezone":"UTC","title":"","origin":"o","goal":"g"}`:                "title",
		`{"name":"n","cron":"* * * * *","timezone":"UTC","title":"t","origin":"o","goal":"g","max_steps":0}`: "max_steps",
	}
	for body, field := range cases {
		_, err := ParseSchedule([]byte(body))
		validation, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("%s: expected a validation error, got %v", body, err)
		}
		if validation.Errors[0].Loc[1] != field {
			t.Fatalf("%s: got field %v, want %s", body, validation.Errors[0].Loc[1], field)
		}
	}
}

func TestParseScheduleDefaults(t *testing.T) {
	payload, err := ParseSchedule([]byte(`{"name":" n ","cron":"0  9  * * *","title":" t ","origin":" o ","goal":" g ","hints":["  a 智库 ",""]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if payload.Timezone != "UTC" || payload.Cron != "0 9 * * *" || payload.Name != "n" || payload.Title != "t" {
		t.Fatalf("unexpected normalization: %+v", payload)
	}
	if !payload.EnabledValue() || !payload.BootstrapValue() {
		t.Fatal("flags should default to true")
	}
	if len(payload.Hints) != 1 || payload.Hints[0] != "a 智库" {
		t.Fatalf("hints not trimmed: %+v", payload.Hints)
	}
}
