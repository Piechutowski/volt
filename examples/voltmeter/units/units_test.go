package units

import "testing"

// TestCommitNormalizesCase proves one commit is one value whatever case it
// is written, read or bound in (spec §types).
func TestCommitNormalizesCase(t *testing.T) {
	c, err := CommitParse("  ABCdef0123 ")
	if err != nil || c != "abcdef0123" {
		t.Fatalf("CommitParse = %q, %v", c, err)
	}
	for _, bad := range []string{"", "abc", "xyz123", "0123456789012345678901234567890123456789ab"} {
		if _, err := CommitParse(bad); err == nil {
			t.Errorf("CommitParse(%q) accepted", bad)
		}
	}
	v, err := Commit("DEADBEEF").Value()
	if err != nil || v != "deadbeef" {
		t.Fatalf("Value = %v, %v", v, err)
	}
	var scanned Commit
	if err := scanned.Scan([]byte("CAFE")); err != nil || scanned != "cafe" {
		t.Fatalf("Scan = %q, %v", scanned, err)
	}
	if Commit("abcdef0123").Short() != "abcdef0" || Commit("abc").Short() != "abc" {
		t.Fatal("Short")
	}
}

// TestMillisString proves the unit switches at ten seconds and the value
// scans from either numeric storage class.
func TestMillisString(t *testing.T) {
	if got := Millis(12.34).String(); got != "12.3 ms" {
		t.Errorf("String = %q", got)
	}
	if got := Millis(12345).String(); got != "12.35 s" {
		t.Errorf("String = %q", got)
	}
	var m Millis
	if err := m.Scan(int64(3)); err != nil || m != 3 {
		t.Fatalf("Scan int64 = %v, %v", m, err)
	}
	if err := m.Scan("x"); err == nil {
		t.Fatal("Scan of text accepted")
	}
}
