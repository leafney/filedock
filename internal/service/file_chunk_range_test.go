package service

import "testing"

func TestParseUploadContentRange(t *testing.T) {
	start, end, total, err := ParseUploadContentRange("bytes 5242880-10485759/11534336")
	if err != nil || start != 5242880 || end != 10485759 || total != 11534336 {
		t.Fatalf("range=%d-%d/%d error=%v", start, end, total, err)
	}
	for _, value := range []string{"", "items 0-1/2", "bytes */2", "bytes 1-0/2", "bytes 0-2/2", "bytes a-b/2", "bytes 0-1/*", "bytes 0-1/0"} {
		if _, _, _, err := ParseUploadContentRange(value); err == nil {
			t.Fatalf("ParseUploadContentRange(%q) unexpectedly succeeded", value)
		}
	}
}

func TestUploadChunkPlanUsesSingleChunkThroughTenMiB(t *testing.T) {
	for _, testCase := range []struct {
		size      int64
		chunkSize int64
		parts     int
	}{
		{size: 1, chunkSize: 1, parts: 1},
		{size: UploadSmallFileLimit, chunkSize: UploadSmallFileLimit, parts: 1},
		{size: UploadSmallFileLimit + 1, chunkSize: UploadChunkSize, parts: 3},
		{size: 15 * 1024 * 1024, chunkSize: UploadChunkSize, parts: 3},
	} {
		chunkSize, parts := UploadChunkPlan(testCase.size)
		if chunkSize != testCase.chunkSize || parts != testCase.parts {
			t.Fatalf("plan(%d)=%d/%d, want %d/%d", testCase.size, chunkSize, parts, testCase.chunkSize, testCase.parts)
		}
	}
}
