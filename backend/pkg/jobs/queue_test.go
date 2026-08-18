package jobs

import (
	"path/filepath"
	"testing"
	"time"

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

func payload(url string) string {
	return `{"url":"` + url + `","bitrate":"320k","order":"normal"}`
}

func TestDequeueAtomic(t *testing.T) {
	st := openTest(t)
	q := New(st)

	for _, id := range []string{"j1", "j2"} {
		if err := q.Enqueue(store.Job{ID: id, Type: "download", Payload: payload("https://open.spotify.com/track/" + id)}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	first, err := q.Dequeue("download")
	if err != nil || first == nil {
		t.Fatalf("premier dequeue = %v, %v", first, err)
	}
	second, err := q.Dequeue("download")
	if err != nil || second == nil {
		t.Fatalf("second dequeue = %v, %v", second, err)
	}
	if first.ID == second.ID {
		t.Fatalf("deux workers ont pris le même job %s — le dequeue n'est pas atomique", first.ID)
	}
	// Un troisième dequeue : file vide.
	if got, err := q.Dequeue("download"); err != nil || got != nil {
		t.Errorf("troisième dequeue = %v, %v; want nil", got, err)
	}

	// Les deux jobs sont bien 'running' en base (réclamés atomiquement).
	jobs, err := st.ListJobs("running", 10)
	if err != nil || len(jobs) != 2 {
		t.Errorf("jobs running = %d, %v; want 2", len(jobs), err)
	}
}

func TestDequeuePriority(t *testing.T) {
	st := openTest(t)
	q := New(st)

	// j_low créé en premier mais priorité 0 ; j_high priorité 5.
	_ = q.Enqueue(store.Job{ID: "j_low", Type: "download", Payload: payload("https://a.example/track/1")})
	_ = q.Enqueue(store.Job{ID: "j_high", Type: "download", Payload: payload("https://b.example/track/2"), Priority: 5})

	got, err := q.Dequeue("download")
	if err != nil || got == nil || got.ID != "j_high" {
		t.Fatalf("dequeue = %v, %v; want j_high (priorité 5)", got, err)
	}
}

func TestRetryBackoff(t *testing.T) {
	st := openTest(t)
	// Base minuscule pour un test rapide : 2^0 × 20 ms, max 3 tentatives.
	q := New(st).WithRetry(20*time.Millisecond, 3)

	if err := q.Enqueue(store.Job{ID: "j1", Type: "download", Payload: payload("https://open.spotify.com/track/j1")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	job, err := q.Dequeue("download")
	if err != nil || job == nil {
		t.Fatalf("dequeue: %v %v", job, err)
	}

	// Échec → nouvelle tentative programmée dans le futur.
	if err := q.ScheduleRetry(job.ID, job.Attempts); err != nil {
		t.Fatalf("ScheduleRetry: %v", err)
	}
	if got, _ := q.Dequeue("download"); got != nil {
		t.Fatalf("dequeue avant retry_at devrait être nil, got %v", got)
	}

	// Après le backoff, le job redevient disponible.
	time.Sleep(60 * time.Millisecond)
	got, err := q.Dequeue("download")
	if err != nil || got == nil || got.ID != "j1" {
		t.Fatalf("dequeue après backoff = %v, %v; want j1", got, err)
	}
	if got.Attempts != 2 {
		t.Errorf("attempts = %d, want 2 (incrémenté à chaque dequeue)", got.Attempts)
	}
}

func TestRetryMaxAttempts(t *testing.T) {
	st := openTest(t)
	q := New(st).WithRetry(20*time.Millisecond, 1) // 1 seule tentative

	_ = q.Enqueue(store.Job{ID: "j1", Type: "download", Payload: payload("https://a.example/track/1")})
	job, err := q.Dequeue("download")
	if err != nil || job == nil {
		t.Fatalf("dequeue: %v %v", job, err)
	}
	// Tentatives épuisées → le job est clôturé 'failed', pas de retry.
	if err := q.ScheduleRetry(job.ID, job.Attempts); err != nil {
		t.Fatalf("ScheduleRetry: %v", err)
	}
	done, _ := st.ListJobs("failed", 10)
	if len(done) != 1 || done[0].ID != "j1" {
		t.Errorf("jobs failed = %+v, want j1", done)
	}
	if got, _ := q.Dequeue("download"); got != nil {
		t.Errorf("dequeue après épuisement = %v, want nil", got)
	}
}

func TestCircuitBreaker(t *testing.T) {
	st := openTest(t)
	q := New(st).WithBreakerCooldown(50 * time.Millisecond)
	const src = "open.spotify.com"

	// 5 échecs consécutifs sur la source → circuit ouvert.
	for i := 0; i < BreakerThreshold; i++ {
		if err := q.Complete("j"+string(rune('a'+i)), src, "timeout"); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}

	// Un job de la source en cooldown n'est PAS réclamé…
	_ = q.Enqueue(store.Job{ID: "blocked", Type: "download", Payload: payload("https://" + src + "/track/1")})
	if got, err := q.Dequeue("download"); err != nil || got != nil {
		t.Fatalf("dequeue source en cooldown = %v, %v; want nil", got, err)
	}
	// …mais un job d'une autre source passe.
	_ = q.Enqueue(store.Job{ID: "ok", Type: "download", Payload: payload("https://example.com/track/2")})
	got, err := q.Dequeue("download")
	if err != nil || got == nil || got.ID != "ok" {
		t.Fatalf("dequeue autre source = %v, %v; want ok", got, err)
	}

	// Après le cooldown, la source bloquée redevient éligible.
	time.Sleep(70 * time.Millisecond)
	got, err = q.Dequeue("download")
	if err != nil || got == nil || got.ID != "blocked" {
		t.Fatalf("dequeue après cooldown = %v, %v; want blocked", got, err)
	}
}

// TestKill9Recovery est le cœur de la DoD M4 : un processus tué (kill -9)
// laisse des jobs 'running' orphelins ; au redémarrage, RequeueOrphaned les
// relance et ils reprennent exactement où ils en étaient.
func TestKill9Recovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "soneph.db")

	// « Premier process » : 3 jobs enfilés, 1 déjà réclamé (running) quand
	// le kill -9 arrive — aucun Complete.
	st1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	q1 := New(st1)
	for _, id := range []string{"j1", "j2", "j3"} {
		if err := q1.Enqueue(store.Job{ID: id, Type: "download", Payload: payload("https://open.spotify.com/track/" + id)}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}
	claimed, err := q1.Dequeue("download")
	if err != nil || claimed == nil {
		t.Fatalf("dequeue: %v %v", claimed, err)
	}
	st1.Close() // kill -9 : pas de Complete, pas de cleanup

	// « Second process » : même base, nouveau store + nouvelle file.
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("réouverture: %v", err)
	}
	defer st2.Close()
	q2 := New(st2)
	if err := q2.RequeueOrphaned(); err != nil {
		t.Fatalf("RequeueOrphaned: %v", err)
	}

	// Les 3 jobs reprennent exactement : plus de running orphelin, chacun
	// réclamable une fois.
	var ids []string
	for i := 0; i < 3; i++ {
		job, err := q2.Dequeue("download")
		if err != nil || job == nil {
			t.Fatalf("dequeue %d = %v, %v; want un job", i, job, err)
		}
		ids = append(ids, job.ID)
		if job.Status != "running" {
			t.Errorf("job %s status = %q, want running", job.ID, job.Status)
		}
	}
	if len(unique(ids)) != 3 {
		t.Errorf("jobs repris = %v, want 3 distincts (reprise exacte)", ids)
	}
	if got, _ := q2.Dequeue("download"); got != nil {
		t.Errorf("dequeue après reprise = %v, want nil", got)
	}
	// Le job qui était 'running' au crash a été relancé (plus d'orphelins).
	running, _ := st2.ListJobs("running", 10)
	if len(running) != 3 { // les 3 sont de nouveau running après re-réclamation
		t.Errorf("jobs running après reprise = %d, want 3", len(running))
	}
}

func unique(ids []string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// TestBroadcast vérifie que chaque transition d'état de la file émet
// « job_update » (WebSocket, M4) : enfilé → running → done, puis le cycle
// d'une nouvelle tentative (queued après retry). La file est visible en
// direct, sans polling.
func TestBroadcast(t *testing.T) {
	st := openTest(t)
	var events []string
	q := New(st).WithBroadcast(func(event string, data interface{}) {
		if event != "job_update" {
			t.Errorf("événement = %q, want job_update", event)
		}
		j, ok := data.(store.Job)
		if !ok {
			t.Fatalf("data = %T, want store.Job", data)
		}
		events = append(events, j.ID+":"+j.Status)
	})

	if err := q.Enqueue(store.Job{ID: "j1", Type: "download", Payload: payload("https://open.spotify.com/track/j1")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	claimed, err := q.Dequeue("download")
	if err != nil || claimed == nil {
		t.Fatalf("dequeue = %v, %v", claimed, err)
	}
	if err := q.Complete(claimed.ID, "open.spotify.com", ""); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	want := []string{"j1:queued", "j1:running", "j1:done"}
	if len(events) != len(want) {
		t.Fatalf("événements = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("événement %d = %q, want %q", i, events[i], want[i])
		}
	}

	// Cycle de nouvelle tentative : queued → running → queued (retry).
	events = nil
	if err := q.Enqueue(store.Job{ID: "j2", Type: "download", Payload: payload("https://open.spotify.com/track/j2")}); err != nil {
		t.Fatalf("Enqueue j2: %v", err)
	}
	claimed2, err := q.Dequeue("download")
	if err != nil || claimed2 == nil {
		t.Fatalf("dequeue j2 = %v, %v", claimed2, err)
	}
	if err := q.ScheduleRetry(claimed2.ID, 1); err != nil {
		t.Fatalf("ScheduleRetry: %v", err)
	}
	want2 := []string{"j2:queued", "j2:running", "j2:queued"}
	if len(events) != len(want2) {
		t.Fatalf("événements retry = %v, want %v", events, want2)
	}
	for i := range want2 {
		if events[i] != want2[i] {
			t.Errorf("événement retry %d = %q, want %q", i, events[i], want2[i])
		}
	}
}
