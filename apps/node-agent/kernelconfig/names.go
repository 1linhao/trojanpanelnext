package kernelconfig

import "regexp"

var (
	xrayConfigNameRegex   = regexp.MustCompile(`^config-([1-9]\d*)-([a-z0-9]+)\.json$`)
	singleConfigNameRegex = regexp.MustCompile(`^config-([1-9]\d*)\.json$`)
)

// ParseXrayConfigName returns the API port and protocol encoded by a generated
// Xray configuration file name.
func ParseXrayConfigName(name string) (string, string, bool) {
	matches := xrayConfigNameRegex.FindStringSubmatch(name)
	if len(matches) != 3 {
		return "", "", false
	}
	return matches[1], matches[2], true
}

// ParseSingleConfigName returns the API port encoded by a generated
// NaiveProxy or Hysteria2 configuration file name.
func ParseSingleConfigName(name string) (string, bool) {
	matches := singleConfigNameRegex.FindStringSubmatch(name)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}
