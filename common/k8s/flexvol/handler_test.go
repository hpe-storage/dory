package flexvol

import "testing"

func TestInitCapabilities(t *testing.T) {
	expectedInitResponse := "{\"status\":\"Success\",\"capabilities\":{\"attach\":false}}"
	result := Handle("init", false, []string{"foo", "bar"})
	if result != expectedInitResponse {
		t.Errorf("init response mismatch. Expected " + expectedInitResponse + " got " + result)
	}
}

func TestInitCapabilitiesWith16(t *testing.T) {
	expectedInitResponse := "{\"status\":\"Success\"}"
	result := Handle("init", true, []string{"foo", "bar"})
	if result != expectedInitResponse {
		t.Errorf("init response mismatch. Expected " + expectedInitResponse + " got " + result)
	}
}

func TestInvalidCommand(t *testing.T) {
	expectedResponse := "{\"status\":\"Not supported\",\"message\":\"Not supported.\"}"
	result := Handle("blahblah", false, []string{"foo", "bar"})
	if result != expectedResponse {
		t.Errorf("unsupported command expected response " + expectedResponse + " got " + result)
	}
}

func TestEnsureArgs(t *testing.T) {
	size := 1
	err := ensureArg("init", []string{"foo", "bar"}, size)
	if err != nil {
		t.Errorf("args size is less than" + string(size))
	}
}
