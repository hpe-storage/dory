package provisioner

import (
	"testing"
)

var annotationTests = []struct {
	name       string
	annotation string
	pvcName    string
	err        bool
	pvName     string
}{
	{"test/no annotation", "", "", false, ""},
	{"test/wrong annotation", "foo.com/" + cloneOfPVC, "cloneMe", false, ""},
	{"test/annotation", "dory/" + cloneOfPVC, "cloneMe", true, ""},
}

func TestPVCCloneOf(t *testing.T) {
	p := getTestProvisioner()

	for _, tc := range annotationTests {
		t.Run(tc.name, func(t *testing.T) {
			claim := getTestPVC()
			if tc.annotation != "" {
				claim.Annotations[tc.annotation] = tc.pvcName
			}

			pvName, err := p.getPVFromPVCAnnotation(claim)
			if (err != nil) != tc.err {
				t.Error(
					"For:", "getPVFromPVCAnnotation",
					"expected:", "No error",
					"got:", err,
				)
			}
			if pvName != tc.pvName {
				t.Error(
					"For:", "pvName",
					"expected:", "",
					"got:", pvName,
				)
			}

		})
	}
}
