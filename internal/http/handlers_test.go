package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"sangfor-log-search/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("change-me"), bcrypt.DefaultCost)
	sessions := auth.NewSessionManager("test-secret-key")

	mux := http.NewServeMux()
	mux.HandleFunc("/login", sessions.LoginHandler("admin", string(hash)))
	mux.HandleFunc("/", sessions.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dashboard"))
	}))

	return httptest.NewServer(mux)
}

func postForm(t *testing.T, url string, vals url.Values) *http.Response {
	t.Helper()
	resp, err := noRedirectClient.PostForm(url, vals)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func getWithCookie(t *testing.T, url string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestLoginSetsSessionCookieAndProtectsQuery(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	resp := postForm(t, srv.URL+"/login", url.Values{
		"username": {"admin"},
		"password": {"change-me"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if len(resp.Cookies()) == 0 {
		t.Fatal("missing session cookie")
	}

	cookie := resp.Cookies()[0]
	if !strings.Contains(cookie.Value, "admin") {
		t.Fatalf("cookie value = %q", cookie.Value)
	}

	resp2 := getWithCookie(t, srv.URL+"/", cookie)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200", resp2.StatusCode)
	}
}

func TestUnauthenticatedRequestRedirects(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	resp, err := noRedirectClient.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 redirect to /login", resp.StatusCode)
	}
}

func TestWrongPasswordRejected(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	resp := postForm(t, srv.URL+"/login", url.Values{
		"username": {"admin"},
		"password": {"wrong"},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
