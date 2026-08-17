// Package jobs gère la file de tâches persistante (table jobs) : dequeue
// atomique, nouvelle tentative avec backoff exponentiel et circuit breaker
// par source (5 échecs consécutifs → 10 min de cooldown). La file survit à
// un kill -9 : au redémarrage, RequeueOrphaned relance les jobs 'running'.
package jobs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"soneph-backend/pkg/store"
)

const (
	// DefaultRetryBase est le délai de base du backoff (2^attempts × base).
	DefaultRetryBase = 30 * time.Second
	// DefaultMaxAttempts borne le nombre total de tentatives par job.
	DefaultMaxAttempts = 3
	// BreakerThreshold : échecs consécutifs avant ouverture du circuit.
	BreakerThreshold = 5
	// BreakerCooldown : durée de refroidissement d'une source ouverte.
	BreakerCooldown = 10 * time.Minute
	// PollInterval : fréquence de scrutation de la file quand elle est vide.
	PollInterval = 500 * time.Millisecond
)

// Queue est la file de tâches sur la table jobs.
type Queue struct {
	st store.Store

	mu              sync.Mutex // sérialise dequeue + breaker (mono-instance)
	breakers        map[string]*breaker
	typesMu         sync.Mutex        // protège types (écrit par Enqueue/Dequeue)
	types           map[string]string // id → type, pour les événements (mono-instance)
	retryBase       time.Duration
	maxAttempts     int
	breakerCooldown time.Duration

	// broadcast notifie les clients connectés (WebSocket) de chaque
	// transition d'état d'un job — M4 : la file est visible en direct,
	// sans polling. Événement émis : « job_update ».
	broadcast func(event string, data interface{})
}

type breaker struct {
	failures      int
	cooldownUntil time.Time
}

// New crée une file sur le store donné avec les réglages par défaut.
func New(st store.Store) *Queue {
	return &Queue{
		st:              st,
		breakers:        map[string]*breaker{},
		types:           map[string]string{},
		retryBase:       DefaultRetryBase,
		maxAttempts:     DefaultMaxAttempts,
		breakerCooldown: BreakerCooldown,
	}
}

// WithBroadcast branche la file sur le hub WebSocket : chaque transition
// d'état (queued → running → done/failed/retry) émet « job_update » avec le
// job concerné.
func (q *Queue) WithBroadcast(fn func(event string, data interface{})) *Queue {
	q.broadcast = fn
	return q
}

// WithRetry ajuste le backoff et le nombre max de tentatives (tests).
func (q *Queue) WithRetry(base time.Duration, maxAttempts int) *Queue {
	q.retryBase = base
	if maxAttempts > 0 {
		q.maxAttempts = maxAttempts
	}
	return q
}

// WithBreakerCooldown ajuste la durée de refroidissement du circuit breaker
// (tests : la valeur par défaut de 10 min rendrait le test très lent).
func (q *Queue) WithBreakerCooldown(d time.Duration) *Queue {
	q.breakerCooldown = d
	return q
}

// PayloadDownload est le payload JSON d'un job de type download.
type PayloadDownload struct {
	URL     string `json:"url"`
	Bitrate string `json:"bitrate"`
	Order   string `json:"order"`
}

// Enqueue ajoute un job à la file.
func (q *Queue) Enqueue(job store.Job) error {
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.Type != "" {
		q.typesMu.Lock()
		q.types[job.ID] = job.Type
		q.typesMu.Unlock()
	}
	if err := q.st.CreateJob(job); err != nil {
		return err
	}
	q.emit(job)
	return nil
}

// emit publie « job_update » aux clients connectés (no-op sans WithBroadcast).
func (q *Queue) emit(job store.Job) {
	if q.broadcast == nil {
		return
	}
	if job.Type == "" {
		q.typesMu.Lock()
		job.Type = q.types[job.ID]
		q.typesMu.Unlock()
	}
	q.broadcast("job_update", job)
}

// Dequeue réclame le prochain job 'queued' du type donné (priorité puis
// ancienneté) dont la source n'est pas en cooldown, et le passe atomiquement
// en 'running'. Retourne (nil, nil) si rien n'est prêt. La réclamation est
// une seule UPDATE…RETURNING — deux workers ne peuvent pas prendre le même
// job ; le mutex rend le contrôle du circuit breaker exempt de course.
func (q *Queue) Dequeue(jobType string) (*store.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Candidats dans l'ordre, en sautant les sources en cooldown.
	rows, err := q.st.ListJobsQueued(jobType, 50)
	if err != nil {
		return nil, fmt.Errorf("jobs: lecture de la file: %w", err)
	}
	for _, j := range rows {
		if q.sourceBlocked(sourceOf(j.Payload)) {
			continue
		}
		claimed, err := q.st.ClaimJob(j.ID)
		if err != nil {
			return nil, fmt.Errorf("jobs: réclamation de %s: %w", j.ID, err)
		}
		if claimed != nil {
			q.typesMu.Lock()
			q.types[claimed.ID] = claimed.Type
			q.typesMu.Unlock()
			q.emit(*claimed) // running
			return claimed, nil
		}
		// Perdu la course (autre worker) : on retente avec le suivant.
	}
	return nil, nil
}

// sourceBlocked reporte si la source est en cooldown (circuit ouvert).
func (q *Queue) sourceBlocked(source string) bool {
	if source == "" {
		return false
	}
	b := q.breakers[source]
	if b == nil {
		return false
	}
	if time.Now().Before(b.cooldownUntil) {
		return true
	}
	// Cooldown écoulé : on réarme le compteur.
	b.failures = 0
	return false
}

// sourceOf extrait le nom d'hôte du payload JSON d'un job.
func sourceOf(payload string) string {
	var p struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil || p.URL == "" {
		return ""
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Complete clôt un job : 'done' ou 'failed' selon errMsg, et met à jour le
// circuit breaker de la source (échec → compteur, succès → réarmement).
func (q *Queue) Complete(jobID, source, errMsg string) error {
	q.mu.Lock()
	if errMsg == "" {
		q.recordSuccessLocked(source)
	} else {
		q.recordFailureLocked(source)
	}
	q.mu.Unlock()

	status := "done"
	if errMsg != "" {
		status = "failed"
	}
	err := q.st.UpdateJobStatus(jobID, status, errMsg)
	if err == store.ErrNotFound {
		// Job déjà absent (base réinitialisée…) : le compteur du breaker,
		// mis à jour plus haut, reste la vérité.
		return nil
	}
	if err == nil {
		q.emit(store.Job{ID: jobID, Status: status, Error: errMsg})
	}
	return err
}

// ScheduleRetry repasse un job en 'queued' avec un retry_at futur calculé
// par backoff exponentiel (2^attempts × base). Si le nombre de tentatives
// est épuisé, le job est clôturé en 'failed'.
func (q *Queue) ScheduleRetry(jobID string, attempts int) error {
	if attempts >= q.maxAttempts {
		if err := q.st.UpdateJobStatus(jobID, "failed", "nombre maximal de tentatives atteint"); err != nil {
			return err
		}
		q.emit(store.Job{ID: jobID, Status: "failed", Error: "nombre maximal de tentatives atteint"})
		return nil
	}
	// attempts est ≥ 1 après le dequeue : première nouvelle tentative = 2^0.
	delay := q.retryBase << (attempts - 1)
	err := q.st.UpdateJobStatus(jobID, "queued", "")
	if err != nil {
		return err
	}
	if err := q.st.SetRetryAt(jobID, time.Now().Add(delay)); err != nil {
		return err
	}
	q.emit(store.Job{ID: jobID, Status: "queued"})
	return nil
}

// RequeueOrphaned relance les jobs 'running' laissés par un processus mort
// (kill -9) : ils repassent 'queued' pour être repris au prochain dequeue.
func (q *Queue) RequeueOrphaned() error {
	n, err := q.st.RequeueRunning()
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("jobs relancés après redémarrage", "count", n)
	}
	return nil
}

func (q *Queue) recordSuccessLocked(source string) {
	if source == "" {
		return
	}
	if b := q.breakers[source]; b != nil {
		b.failures = 0
	}
}

func (q *Queue) recordFailureLocked(source string) {
	if source == "" {
		return
	}
	b := q.breakers[source]
	if b == nil {
		b = &breaker{}
		q.breakers[source] = b
	}
	b.failures++
	if b.failures >= BreakerThreshold {
		b.cooldownUntil = time.Now().Add(q.breakerCooldown)
		b.failures = 0
		slog.Warn("circuit breaker ouvert pour la source", "source", source, "cooldown", q.breakerCooldown)
	}
}
