package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Metadata struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func Current() Metadata {
	return Metadata{
		Version:   valueOrDefault(Version, "dev"),
		Commit:    valueOrDefault(Commit, "unknown"),
		BuildDate: valueOrDefault(BuildDate, "unknown"),
	}
}

func Summary() string {
	info := Current()
	return fmt.Sprintf("Tanabata %s (%s) built %s", info.Version, info.Commit, info.BuildDate)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
