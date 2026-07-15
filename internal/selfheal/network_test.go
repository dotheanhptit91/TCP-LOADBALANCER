package selfheal

import (
	"reflect"
	"testing"
)

func TestParseInterfaces(t *testing.T) {
	got := parseInterfaces(" worker1,worker2, worker1, server ")
	want := []string{"worker1", "worker2", "server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseInterfaces() = %v, want %v", got, want)
	}
}

func TestMissingInterfaces(t *testing.T) {
	missing := missingInterfaces([]string{"lo", "interface-that-does-not-exist"})
	if !reflect.DeepEqual(missing, []string{"interface-that-does-not-exist"}) {
		t.Fatalf("missingInterfaces() = %v", missing)
	}
}
