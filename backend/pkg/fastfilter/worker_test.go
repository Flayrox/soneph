package fastfilter

import (
	"path/filepath"
	"testing"

	"soneph-backend/pkg/jobs"
	"soneph-backend/pkg/store"
)

// openTest ouvre une base SQLite temporaire réelle (migrations appliquées).
func openTest(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "soneph.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestWorkerEndToEnd vérifie le cycle complet d'un job fast_filter dans la
// file M4 : réclamation atomique → filtrage Go (injecté) → clôture 'done' →
// callback qui ré-enfile le téléchargement avec le même task_id.
func TestWorkerEndToEnd(t *testing.T) {
	st := openTest(t)
	q := jobs.New(st)

	var gotPayload Payload
	var gotRes Result
	worker := NewWorker(q, t.TempDir(), func(p Payload, res Result) {
		gotPayload = p
		gotRes = res
	}).WithRun(func(dir, mediaURL string, fetch FetchFunc) Result {
		if dir == "" {
			t.Error("downloadDir vide transmis au filtre")
		}
		return Result{Applied: true, TotalTracks: 2, AlreadyDownloaded: 1, MissingCount: 1,
			SkippedTracks: []string{"A - déjà là"}, MissingQueries: []string{"B - à télécharger"}}
	})

	rawPayload := `{"task_id":"task_1","url":"https://open.spotify.com/playlist/abc","bitrate":"320k","order":"normal"}`
	if err := q.Enqueue(store.Job{ID: "ff_task_1", Type: "fast_filter", Payload: rawPayload}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	job, err := q.Dequeue("fast_filter")
	if err != nil || job == nil {
		t.Fatalf("dequeue fast_filter = %v, %v", job, err)
	}
	worker.process(job)

	// Le job est clôturé 'done' et le callback a reçu payload + résultat.
	done, err := st.ListJobs("done", 10)
	if err != nil || len(done) != 1 || done[0].ID != "ff_task_1" {
		t.Fatalf("jobs done = %+v, %v; want ff_task_1", done, err)
	}
	if gotPayload.TaskID != "task_1" || gotPayload.URL != "https://open.spotify.com/playlist/abc" {
		t.Errorf("payload reçu = %+v", gotPayload)
	}
	if !gotRes.Applied || gotRes.TotalTracks != 2 || gotRes.MissingCount != 1 {
		t.Errorf("résultat reçu = %+v", gotRes)
	}

	// Le callback (wired dans main.go) ré-enfile le téléchargement : le job
	// download avec le task_id d'origine doit être réclamable.
	if err := q.Enqueue(store.Job{ID: gotPayload.TaskID, Type: "download", Payload: `{"url":"https://open.spotify.com/playlist/abc"}`}); err != nil {
		t.Fatalf("ré-enfilage download: %v", err)
	}
	dl, err := q.Dequeue("download")
	if err != nil || dl == nil || dl.ID != "task_1" {
		t.Fatalf("dequeue download = %v, %v; want task_1", dl, err)
	}
}

// TestWorkerInvalidPayload : un payload illisible clôture le job en échec
// sans appeler le callback (rien à ré-enfiler).
func TestWorkerInvalidPayload(t *testing.T) {
	st := openTest(t)
	q := jobs.New(st)
	called := false
	worker := NewWorker(q, t.TempDir(), func(Payload, Result) { called = true })

	_ = q.Enqueue(store.Job{ID: "ff_bad", Type: "fast_filter", Payload: "{not json"})
	job, err := q.Dequeue("fast_filter")
	if err != nil || job == nil {
		t.Fatalf("dequeue = %v, %v", job, err)
	}
	worker.process(job)

	failed, err := st.ListJobs("failed", 10)
	if err != nil || len(failed) != 1 || failed[0].ID != "ff_bad" {
		t.Fatalf("jobs failed = %+v, %v; want ff_bad", failed, err)
	}
	if called {
		t.Error("callback appelé pour un payload invalide")
	}
}

// TestWorkerJobUpdate : le worker passe par la file M4, donc chaque
// transition émet « job_update » (queued → running → done) sur le WebSocket.
func TestWorkerJobUpdate(t *testing.T) {
	st := openTest(t)
	var events []string
	q := jobs.New(st).WithBroadcast(func(event string, data interface{}) {
		j := data.(store.Job)
		events = append(events, j.ID+":"+j.Status)
	})
	worker := NewWorker(q, t.TempDir(), func(Payload, Result) {}).
		WithRun(func(string, string, FetchFunc) Result { return Result{Applied: true} })

	_ = q.Enqueue(store.Job{ID: "ff_1", Type: "fast_filter", Payload: `{"task_id":"t1","url":"https://open.spotify.com/playlist/abc"}`})
	job, _ := q.Dequeue("fast_filter")
	worker.process(job)

	want := []string{"ff_1:queued", "ff_1:running", "ff_1:done"}
	if len(events) != len(want) {
		t.Fatalf("événements = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("événement %d = %q, want %q", i, events[i], want[i])
		}
	}
}
