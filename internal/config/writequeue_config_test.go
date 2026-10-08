package config

import "testing"

func TestWriteQueueConstantsMatchSpec(t *testing.T) {
	t.Parallel()
	if WriteQueueApplyTimeoutMs != 30_000 {
		t.Errorf("WriteQueueApplyTimeoutMs = %d, want 30000", WriteQueueApplyTimeoutMs)
	}
	if WriteQueueDepth != 256 {
		t.Errorf("WriteQueueDepth = %d, want 256", WriteQueueDepth)
	}
}
