package main

import (
	"reflect"
	"testing"
)

func TestParseFlagsAnywhere(t *testing.T) {
	cases := [][]string{
		{"nes", "--json"},
		{"--json", "nes"},
		{"-json", "nes", "--", "--literal"},
	}
	want := [][]string{{"nes"}, {"nes"}, {"nes", "--literal"}}
	for i, args := range cases {
		fs, asJSON := newFlags("t")
		pos, err := parse(fs, args)
		if err != nil {
			t.Fatal(err)
		}
		if !*asJSON || !reflect.DeepEqual(pos, want[i]) {
			t.Errorf("parse(%q) = %q json=%v, want %q json=true", args, pos, *asJSON, want[i])
		}
	}
}
