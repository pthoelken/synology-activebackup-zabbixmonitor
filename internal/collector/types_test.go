package collector

import "testing"

func TestStatusFromRawABBPartialSuccessIsWarning(t *testing.T) {
	for _, raw := range []string{"8", "partial success", "partially completed", "not fully completed"} {
		if got := StatusFromRaw(ProductABB, raw); got != StatusWarning {
			t.Fatalf("StatusFromRaw(ProductABB, %q) = %d, want %d", raw, got, StatusWarning)
		}
	}
}

func TestStatusFromRawABBSuccessfulIsOK(t *testing.T) {
	for _, raw := range []string{"2", "successful"} {
		if got := StatusFromRaw(ProductABB, raw); got != StatusOK {
			t.Fatalf("StatusFromRaw(ProductABB, %q) = %d, want %d", raw, got, StatusOK)
		}
	}
}

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
