package service

import "testing"

func TestGitHubReleaseResponseBoundsCoverBothChannels(t *testing.T) {
	if githubReleasePageSize < 20 {
		t.Fatalf("release page must leave room for both channels, got %d", githubReleasePageSize)
	}
	if githubReleaseResponseSize < 8<<20 {
		t.Fatalf("release response limit is too small: %d", githubReleaseResponseSize)
	}
}

func TestFilterKernelReleasesStrictlyUsesPrereleaseFlag(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v1.0.0", Prerelease: false},
		{TagName: "v1.1.0-beta.1", Prerelease: true},
		{TagName: "v1.0.1", Prerelease: false, Draft: true},
	}
	stable := filterKernelReleases(releases, "stable", 10, "xray")
	if len(stable) != 1 || stable[0].Version != "v1.0.0" {
		t.Fatalf("stable releases = %#v", stable)
	}
	prerelease := filterKernelReleases(releases, "prerelease", 10, "xray")
	if len(prerelease) != 1 || prerelease[0].Version != "v1.1.0-beta.1" {
		t.Fatalf("prereleases = %#v", prerelease)
	}
}

func TestFilterKernelReleasesNormalizesHysteriaAppTag(t *testing.T) {
	releases := []githubRelease{
		{TagName: "app/v2.10.0"},
		{TagName: "v1.3.5"},
	}
	stable := filterKernelReleases(releases, "stable", 10, "hysteria2")
	if len(stable) != 1 || stable[0].Version != "v2.10.0" {
		t.Fatalf("hysteria2 releases = %#v", stable)
	}
}
