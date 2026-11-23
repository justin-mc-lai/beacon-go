package orchestrator

import (
	"reflect"
	"testing"
)

func TestTaskGoVerificationArgs(t *testing.T) {
	got := taskGoVerificationArgv([]string{"go", "test", "-json", "./pkg", "-count=8", "-args", "-config", "fixture.json"})
	want := []string{"go", "test", "./pkg", "-v", "-count=1", "-args", "-config", "fixture.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%q want %q", got, want)
	}
}
