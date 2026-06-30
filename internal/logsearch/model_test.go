package logsearch

import "testing"

func TestBuildSearchTextAddsCanonicalIPTokens(t *testing.T) {
	got := BuildSearchText("src=10.10.10.1 dst=192.168.1.8 action=deny")

	for _, want := range []string{"ip_10_10_10_1", "ip_192_168_1_8", "src=10.10.10.1"} {
		if !contains(got, want) {
			t.Fatalf("BuildSearchText() = %q，缺少 %q", got, want)
		}
	}
}

func TestDateKeyFromFilenameUsesLastDateInLogName(t *testing.T) {
	got := DateKeyFromFilename("10.10.10.1_2026-04-28.log-20260429.gz")
	if got != 20260429 {
		t.Fatalf("DateKeyFromFilename() = %d，期望 20260429", got)
	}
}

func TestNormalizeDateKeysSupportYearMonthAndDay(t *testing.T) {
	start, err := NormalizeDateKey("202604")
	if err != nil {
		t.Fatal(err)
	}
	end, err := NormalizeEndDateKey("202604")
	if err != nil {
		t.Fatal(err)
	}
	if start != 20260401 || end != 20260431 {
		t.Fatalf("日期归一化错误：start=%d end=%d", start, end)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

