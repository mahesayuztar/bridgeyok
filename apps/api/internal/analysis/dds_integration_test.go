//go:build integration

package analysis

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestPinnedDDSGoldenFixtures(t *testing.T) {
	path, err := filepath.Abs("../../../../bin/bridgeyok-dds")
	if err != nil {
		t.Fatal(err)
	}
	solver, err := NewDDS(path, 10*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range goldenFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			metadata, err := bridge.MetadataForBoard(fixture.BoardNumber)
			if err != nil {
				t.Fatal(err)
			}
			result, err := solver.Solve(t.Context(), fixture.Deal, metadata)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, fixture.Expected) {
				t.Fatalf("DDS golden mismatch\ngot %+v\nwant %+v", result, fixture.Expected)
			}
		})
	}
}
