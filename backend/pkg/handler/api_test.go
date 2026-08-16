package handler

import "testing"

func TestIsPlaylistLink(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", true},
		{"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", true},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=abc123", true},
		{"https://open.spotify.com/album/0hl8Tcf7Ik3", false},
		{"https://open.spotify.com/track/5ojN1zP5JN", false},
		{"https://open.spotify.com/artist/6fxyWrfmjcbj5d12gXeiNV", false},
		{"https://open.spotify.com/playlist/", true},
		{"", false},
	}
	for _, c := range cases {
		if got := isPlaylistLink(c.url); got != c.want {
			t.Errorf("isPlaylistLink(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
