package commands

import (
	"fmt"
	"runtime"
)

type VersionInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

func Version(info VersionInfo) string {
	if info.Version == "" {
		info.Version = "dev"
	}
	return fmt.Sprintf("sigilbridge %s commit=%s date=%s go=%s", info.Version, valueOr(info.Commit, "unknown"), valueOr(info.BuildDate, "unknown"), runtime.Version())
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
