package parser

import (
	"fmt"
	"os"

	"github.com/google/pprof/profile"
)

// Load reads and parses a pprof profile from path.
// Both gzip-compressed and uncompressed profiles are supported.
func Load(path string) (*profile.Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open profile: %w", err)
	}
	defer f.Close()

	p, err := profile.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse profile %s: %w", path, err)
	}
	return p, nil
}

// ValueIndex returns the index in Sample.Value for the given type name.
// The second return value is false if the type is not present in the profile.
func ValueIndex(p *profile.Profile, typeName string) (int, bool) {
	for i, st := range p.SampleType {
		if st.Type == typeName {
			return i, true
		}
	}
	return -1, false
}

// DetectType guesses the profile type from its sample type names.
func DetectType(p *profile.Profile) string {
	for _, st := range p.SampleType {
		switch st.Type {
		case "alloc_space", "inuse_space":
			return "heap"
		case "cpu":
			return "cpu"
		case "goroutine":
			return "goroutine"
		case "contentions", "delay":
			return "mutex"
		}
	}
	return "unknown"
}
