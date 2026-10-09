package version

import (
	"regexp"
	"testing"
)

func TestStringIsCalendarSemver(t *testing.T) {
	// The literal changes every commit, so pin the shape, not the value.
	if !regexp.MustCompile(`^v20\d\d\.([1-9]|1[0-2])\.\d+$`).MatchString(String()) {
		t.Fatalf("version %q is not vYEAR.MONTH.PATCH with an unpadded month", String())
	}
}
