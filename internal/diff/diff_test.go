package diff

import (
	"testing"

	"github.com/google/pprof/profile"
)

func makeProfile(sampleType string, samples []struct {
	fn    string
	value int64
}) *profile.Profile {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: sampleType, Unit: "bytes"}},
	}
	for _, s := range samples {
		fn := &profile.Function{ID: uint64(len(p.Function)) + 1, Name: s.fn}
		p.Function = append(p.Function, fn)
		loc := &profile.Location{
			ID:   uint64(len(p.Location)) + 1,
			Line: []profile.Line{{Function: fn}},
		}
		p.Location = append(p.Location, loc)
		p.Sample = append(p.Sample, &profile.Sample{
			Value:    []int64{s.value},
			Location: []*profile.Location{loc},
		})
	}
	return p
}

func TestCompare_Increased(t *testing.T) {
	before := makeProfile("alloc_space", []struct {
		fn    string
		value int64
	}{
		{"pkg.Foo", 100},
		{"pkg.Bar", 200},
	})
	after := makeProfile("alloc_space", []struct {
		fn    string
		value int64
	}{
		{"pkg.Foo", 300},
		{"pkg.Bar", 200},
	})

	r, err := Compare(before, after, 10)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(r.Increased) != 1 {
		t.Fatalf("expected 1 increased entry, got %d", len(r.Increased))
	}
	if r.Increased[0].Function != "pkg.Foo" {
		t.Errorf("expected pkg.Foo in increased, got %s", r.Increased[0].Function)
	}
	if r.Increased[0].Delta != 200 {
		t.Errorf("expected delta=200, got %d", r.Increased[0].Delta)
	}
}

func TestCompare_NewAndGone(t *testing.T) {
	before := makeProfile("alloc_space", []struct {
		fn    string
		value int64
	}{
		{"pkg.Old", 500},
	})
	after := makeProfile("alloc_space", []struct {
		fn    string
		value int64
	}{
		{"pkg.New", 300},
	})

	r, err := Compare(before, after, 10)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(r.New) != 1 || r.New[0].Function != "pkg.New" {
		t.Errorf("expected pkg.New in new, got %v", r.New)
	}
	if len(r.Gone) != 1 || r.Gone[0].Function != "pkg.Old" {
		t.Errorf("expected pkg.Old in gone, got %v", r.Gone)
	}
}
