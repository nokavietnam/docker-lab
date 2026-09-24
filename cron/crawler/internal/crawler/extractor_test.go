package crawler

import (
	"testing"
)

func TestExtractListings(t *testing.T) {
	mockHTML := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Muaban</h1>
<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"classified":{"total":2,"items":[{"id":1001,"title":"Nhà quận 1","price":5000000000,"price_display":"5 tỷ","location":"Quận 1, TP.HCM"},{"id":1002,"title":"Nhà quận 3","price":7000000000,"price_display":"7 tỷ","location":"Quận 3, TP.HCM"}]}}}}</script>
</body>
</html>`

	items, total, err := ExtractListings([]byte(mockHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != 1001 || items[0].Title != "Nhà quận 1" || items[0].Price != 5000000000 {
		t.Errorf("item 0 content mismatch: %+v", items[0])
	}

	if items[1].ID != 1002 || items[1].Title != "Nhà quận 3" || items[1].Price != 7000000000 {
		t.Errorf("item 1 content mismatch: %+v", items[1])
	}
}

func TestExtractListings_NotFound(t *testing.T) {
	mockHTML := `<html><body>No next data here</body></html>`
	_, _, err := ExtractListings([]byte(mockHTML))
	if err != ErrNextDataNotFound {
		t.Fatalf("expected ErrNextDataNotFound, got %v", err)
	}
}
