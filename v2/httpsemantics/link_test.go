package httpsemantics

import (
	"net/url"
	"testing"
)

func TestLinkFormat_Simple(t *testing.T) {
	l := Link{URI: "/users", Rel: "self"}
	got := l.Format()
	want := `</users>; rel="self"`
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestLinkFormat_WithTitle(t *testing.T) {
	l := Link{URI: "/users", Rel: "next", Title: "Next Page"}
	got := l.Format()
	want := `</users>; rel="next"; title="Next Page"`
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestLinkFormat_WithType(t *testing.T) {
	l := Link{URI: "/users.json", Rel: "self", Type: "application/json"}
	got := l.Format()
	if got != `</users.json>; rel="self"; type="application/json"` {
		t.Errorf("Format() = %q", got)
	}
}

func TestFormatLinkHeader_Multiple(t *testing.T) {
	links := []Link{
		{URI: "/page/1", Rel: "first"},
		{URI: "/page/2", Rel: "next"},
		{URI: "/page/10", Rel: "last"},
	}
	got := FormatLinkHeader(links)
	want := `</page/1>; rel="first", </page/2>; rel="next", </page/10>; rel="last"`
	if got != want {
		t.Errorf("FormatLinkHeader() = %q, want %q", got, want)
	}
}

func TestBuildPaginationLinks_MiddlePage(t *testing.T) {
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:    "/api/users",
		Page:       3,
		PageSize:   10,
		TotalItems: 50,
	})
	// Should have: self (omitted since Self is empty), first, prev, next, last
	// Actually self is omitted when empty
	if len(links) != 4 {
		t.Fatalf("len = %d, want 4 (first, prev, next, last)", len(links))
	}
	rels := []string{links[0].Rel, links[1].Rel, links[2].Rel, links[3].Rel}
	expected := []string{"first", "prev", "next", "last"}
	for i, want := range expected {
		if rels[i] != want {
			t.Errorf("links[%d].Rel = %q, want %q", i, rels[i], want)
		}
	}
}

func TestBuildPaginationLinks_FirstPage(t *testing.T) {
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:    "/api/users",
		Page:       1,
		PageSize:   10,
		TotalItems: 50,
	})
	// Should have: first, next, last (no prev)
	if len(links) != 3 {
		t.Fatalf("len = %d, want 3 (first, next, last)", len(links))
	}
	for _, l := range links {
		if l.Rel == "prev" {
			t.Error("prev should not be present on page 1")
		}
	}
}

func TestBuildPaginationLinks_LastPage(t *testing.T) {
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:    "/api/users",
		Page:       5,
		PageSize:   10,
		TotalItems: 50,
	})
	// Should have: first, prev, last (no next)
	for _, l := range links {
		if l.Rel == "next" {
			t.Error("next should not be present on last page")
		}
	}
}

func TestBuildPaginationLinks_UnknownTotal(t *testing.T) {
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:    "/api/users",
		Page:       1,
		PageSize:   10,
		TotalItems: 0,
	})
	// Should have: first, next (no last since total is unknown)
	for _, l := range links {
		if l.Rel == "last" {
			t.Error("last should not be present when total is unknown")
		}
	}
}

func TestBuildPaginationLinks_PreservesExtraParams(t *testing.T) {
	extra := url.Values{}
	extra.Set("sort", "name")
	extra.Set("filter", "active")
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:     "/api/users",
		Page:        1,
		PageSize:    10,
		TotalItems:  20,
		ExtraParams: extra,
	})
	for _, l := range links {
		if l.Rel == "next" {
			parsed, err := url.Parse(l.URI)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if parsed.Query().Get("sort") != "name" {
				t.Errorf("sort = %q, want %q", parsed.Query().Get("sort"), "name")
			}
			if parsed.Query().Get("filter") != "active" {
				t.Errorf("filter = %q, want %q", parsed.Query().Get("filter"), "active")
			}
		}
	}
}

func TestBuildPaginationLinks_CustomParamNames(t *testing.T) {
	links := BuildPaginationLinks(PaginationConfig{
		BaseURL:       "/api/users",
		Page:          2,
		PageSize:      20,
		TotalItems:    100,
		PageParam:     "p",
		PageSizeParam: "size",
	})
	for _, l := range links {
		if l.Rel == "first" {
			parsed, err := url.Parse(l.URI)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if parsed.Query().Get("p") != "1" {
				t.Errorf("p = %q, want %q", parsed.Query().Get("p"), "1")
			}
			if parsed.Query().Get("size") != "20" {
				t.Errorf("size = %q, want %q", parsed.Query().Get("size"), "20")
			}
		}
	}
}

func TestParseLinkHeader_Simple(t *testing.T) {
	links, err := ParseLinkHeader(`</users>; rel="self"`)
	if err != nil {
		t.Fatalf("ParseLinkHeader() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].URI != "/users" {
		t.Errorf("URI = %q, want %q", links[0].URI, "/users")
	}
	if links[0].Rel != "self" {
		t.Errorf("Rel = %q, want %q", links[0].Rel, "self")
	}
}

func TestParseLinkHeader_Multiple(t *testing.T) {
	links, err := ParseLinkHeader(`</page/1>; rel="first", </page/2>; rel="next"`)
	if err != nil {
		t.Fatalf("ParseLinkHeader() error = %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
}

func TestParseLinkHeader_Empty(t *testing.T) {
	links, err := ParseLinkHeader("")
	if err != nil {
		t.Fatalf("ParseLinkHeader() error = %v", err)
	}
	if links != nil {
		t.Errorf("links = %v, want nil", links)
	}
}

func TestEscapeLinkParam_PreventsInjection(t *testing.T) {
	got := escapeLinkParam("evil\r\nX-Injected: yes")
	if got != "evil  X-Injected: yes" {
		t.Errorf("escapeLinkParam() = %q, want %q", got, "evil  X-Injected: yes")
	}
}
