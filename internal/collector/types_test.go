package collector

import "testing"

func TestStatusFromRawM365SkippedItemsAreWarning(t *testing.T) {
	if got := StatusFromRaw(ProductM365, "6"); got != StatusWarning {
		t.Fatalf("StatusFromRaw(ProductM365, 6) = %d, want %d", got, StatusWarning)
	}
}

func TestStatusFromRawM365Failed(t *testing.T) {
	for _, raw := range []string{"4", "failed", "error", "canceled"} {
		if got := StatusFromRaw(ProductM365, raw); got != StatusFailed {
			t.Fatalf("StatusFromRaw(ProductM365, %q) = %d, want %d", raw, got, StatusFailed)
		}
	}
}
