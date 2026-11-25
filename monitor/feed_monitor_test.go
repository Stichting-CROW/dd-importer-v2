package monitor

import "testing"

func TestShouldCheckDowntime(t *testing.T) {
	tests := []struct {
		name string
		feed FeedStatus
		want bool
	}{
		{"active vehicles feed", FeedStatus{DataType: "vehicles", IsActive: true}, true},
		{"inactive vehicles feed", FeedStatus{DataType: "vehicles", IsActive: false}, false},
		{"active trips feed", FeedStatus{DataType: "trips", IsActive: true}, false},
		{"active service area feed", FeedStatus{DataType: "service_areas", IsActive: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCheckDowntime(tt.feed); got != tt.want {
				t.Errorf("shouldCheckDowntime(%+v) = %v, want %v", tt.feed, got, tt.want)
			}
		})
	}
}
