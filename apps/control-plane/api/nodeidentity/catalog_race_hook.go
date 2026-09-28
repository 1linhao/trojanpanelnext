//go:build !nodeidentitycatalogracetest

package nodeidentity

func catalogRaceBeforeLock(string) error    { return nil }
func catalogRaceAfterPrecheck(string) error { return nil }
