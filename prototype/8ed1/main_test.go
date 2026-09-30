package main

import (
	"os"
	"strings"
	"testing"
)

const weather = "/home/nbyoung/Projects/Tableaux/tableaux/corpus/build/weather-station"

func corpus(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(weather); err != nil {
		t.Skip("the conformance corpus is absent")
	}
}

func spec(t *testing.T, a, b string) string {
	t.Helper()
	return resolve(weather, a) + ".." + resolve(weather, b)
}

func TestHistoryRange(t *testing.T) {
	corpus(t)
	evs, err := History(weather, spec(t, "W2", "W6"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range evs {
		got = append(got, e.Commit+" "+e.Task+" "+e.Event)
	}
	// The corpus build fixes the hashes; the labels file names them.
	h := func(l string) string { return resolve(weather, l)[:7] }
	want := h("W3") + " 9f31 task," + h("W4") + " 9f31 authorised," + h("W5") + " 3c5d task," + h("W6") + " 3c5d authorised"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %v", got)
	}
}

func TestHistoryStatusAndPin(t *testing.T) {
	corpus(t)
	evs, err := History(weather, spec(t, "W9", "W13"))
	if err != nil {
		t.Fatal(err)
	}
	var status, pin, reaffirmed bool
	for _, e := range evs {
		switch {
		case e.Event == "status" && e.Task == "9f31" && e.Gate == "function" && e.State == "stalled" && e.Reason == "blocked":
			status = e.Note == "Barometer ICs on 14-week backorder"
		case e.Event == "pin" && e.Task == "c07d":
			pin = true
		case e.Event == "reaffirmed" && e.Task == "9f31":
			reaffirmed = true
		}
	}
	if !status || !pin || !reaffirmed {
		t.Fatalf("status %v pin %v reaffirmed %v", status, pin, reaffirmed)
	}
}

func TestHistoryOrderedByAuthorTime(t *testing.T) {
	corpus(t)
	evs, err := History(weather, resolve(weather, "W13"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(evs); i++ {
		if evs[i].at < evs[i-1].at {
			t.Fatalf("event %d precedes event %d", i, i-1)
		}
	}
}

func TestAuditRange(t *testing.T) {
	corpus(t)
	fs, err := Audit(weather, resolve(weather, "W1"), resolve(weather, "W13"), 3)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, f := range fs {
		found[f.Rule+"|"+f.Task] = f.Range
	}
	if found["STALE|3c5d"] != "introduced" {
		t.Fatalf("got %v", found)
	}
	if _, ok := found["PROPOSED|c07d"]; ok {
		t.Fatal("c07d is authorised by the assignee of its parent on the trunk")
	}
}
