// Package stubapi serves an in-process stand-in for the Liftoff tRPC API.
//
// It exists so tests can exercise the real data path — flag parsing,
// fetch, filter, render — without a Liftoff account, a stored OAuth
// token, or network access. Point LIFTOFF_API_BASE at Server.BaseURL()
// and set LIFTOFF_REFRESH_TOKEN to any non-empty value; the CLI's
// headless auth path then refreshes against this server instead of
// getgymbros.com.
//
// Nothing in the shipped binary imports this package: `main` reaches
// cmd and internal/{auth,client} only, so stubapi is linked into test
// binaries alone. It deliberately does not import "testing" so both
// the package-level `cmd` tests and the compat-tagged entry point
// (which must hand a URL to an out-of-process child) can use it.
//
// The stub implements the two procedures the data path calls and
// nothing else: an unknown procedure returns a tRPC-shaped error so a
// new call site fails loudly here rather than silently returning zero
// rows.
package stubapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// Server is a running stub. Close it when the test is done.
type Server struct {
	httpSrv *httptest.Server

	mu    sync.RWMutex
	posts json.RawMessage
}

// New starts a stub serving posts as the payload of post.getMyPosts.
// posts must be a JSON array; pass EmptyPosts for the empty-account
// case or SamplePosts for a single realistic workout.
func New(posts json.RawMessage) *Server {
	s := &Server{posts: posts}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/trpc/", s.handleTRPC)
	s.httpSrv = httptest.NewServer(mux)
	return s
}

// BaseURL is the value to assign to LIFTOFF_API_BASE.
func (s *Server) BaseURL() string { return s.httpSrv.URL }

// Close shuts the stub down.
func (s *Server) Close() { s.httpSrv.Close() }

// SetPosts swaps the post.getMyPosts payload. Safe to call while the
// server is running; the `cmd` tests reuse one server across cases
// because internal/auth resolves LIFTOFF_API_BASE exactly once per
// process.
func (s *Server) SetPosts(posts json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posts = posts
}

func (s *Server) handleTRPC(w http.ResponseWriter, r *http.Request) {
	// The client uses the tRPC batch envelope, so the procedure name is
	// the last path segment and the response is a one-element array.
	procedure := strings.TrimPrefix(r.URL.Path, "/api/trpc/")
	w.Header().Set("Content-Type", "application/json")

	switch procedure {
	case "user.refreshToken":
		expires := time.Now().Add(time.Hour).Format(time.RFC3339Nano)
		fmt.Fprintf(w, `[{"result":{"data":{"json":{"accessToken":"stub-access-token","accessTokenExpiresAt":%q}}}}]`, expires)
	case "post.getMyPosts":
		s.mu.RLock()
		posts := s.posts
		s.mu.RUnlock()
		fmt.Fprintf(w, `[{"result":{"data":{"json":%s}}}]`, posts)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `[{"error":{"json":{"message":"stubapi: unimplemented procedure %q"}}}]`, procedure)
	}
}

// EmptyPosts is an account with no workouts logged — the CONTRACT §4
// empty-result case.
var EmptyPosts = json.RawMessage(`[]`)

// SamplePosts is one realistic workout: a weight/reps exercise with two
// identical sets plus a recorded bodyweight, which is enough for every
// data-producing subcommand (`workouts list`, `workouts stats`,
// `bodyweights list`, `bodyweights stats`) to render a non-empty result.
//
// The timestamp is mid-day UTC so a renderer that buckets by local day
// lands on 2026-06-08 for any offset within ±11 hours, keeping the
// fixture stable across CI timezones.
var SamplePosts = json.RawMessage(`[
  {
    "id": "stub-post-1",
    "startedAt": "2026-06-08T12:00:00.000Z",
    "postedAt": "2026-06-08T13:10:00.000Z",
    "sessionDuration": "01 hours 06 minutes 01 seconds",
    "sessionNotes": "",
    "bodyweight": "175",
    "caloriesBurned": 420,
    "prCount": 1,
    "exerciseData": [
      {
        "exerciseIndex": 0,
        "exerciseName": "Bench Press",
        "exerciseId": "stub-ex-1",
        "exerciseTypes": "WR",
        "exerciseNotes": "",
        "setsData": [
          {"setIndex": 0, "setType": "normal", "inputOne": 100, "inputTwo": 5},
          {"setIndex": 1, "setType": "normal", "inputOne": 100, "inputTwo": 5}
        ]
      }
    ]
  }
]`)
