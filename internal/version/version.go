package version

import (
	"fmt"
	"strings"
)

var (
	Version       = "dev"
	Commit        = "unknown"
	PublicVersion = "latest"
)

func Public() string {
	return public(PublicVersion)
}

func public(publicVersion string) string {
	publicVersion = strings.TrimSpace(publicVersion)
	if publicVersion == "" {
		return "latest"
	}
	return publicVersion
}

func Release() string {
	return release(Version, Commit)
}

func release(version, commit string) string {
	version = strings.TrimSpace(version)
	if version != "" && version != "dev" {
		return version
	}

	if short := shortCommit(commit); short != "" {
		return "sha-" + short
	}

	return "dev"
}

func ShortCommit() string {
	return shortCommit(Commit)
}

func shortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" || commit == "unknown" {
		return ""
	}
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func CommandOutput() string {
	return commandOutput(Version, Commit)
}

func commandOutput(version, commit string) string {
	release := release(version, commit)
	commit = strings.TrimSpace(commit)
	if commit == "" || commit == "unknown" {
		return release
	}
	return fmt.Sprintf("%s\ncommit %s", release, commit)
}
