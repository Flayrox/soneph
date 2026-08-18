package fastfilter

import (
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"soneph-backend/pkg/jobs"
	"soneph-backend/pkg/store"
)

// Payload est le payload JSON d'un job de type fast_filter (M5) : le filtre
// tourne en asynchrone dans la file M4, AVANT le téléchargement. À son
// terme, le worker ré-enfile le job download avec le même task_id.
type Payload struct {
	TaskID  string `json:"task_id"`
	URL     string `json:"url"`
	Bitrate string `json:"bitrate"`
	Order   string `json:"order"`
}

// RunFunc exécute le filtre (injectable dans les tests — pas de réseau).
type RunFunc func(dir, mediaURL string, fetch FetchFunc) Result

// OnFiltered est appelé après la clôture réussie d'un job fast_filter :
// le résultat est appliqué à la tâche et le téléchargement est ré-enfilé.
type OnFiltered func(p Payload, res Result)

// Worker consomme les jobs fast_filter de la file M4. Il partage la même
// file (dequeue atomique, backoff, circuit breaker) que le worker download
// — chaque transition d'état est poussée sur le WebSocket (job_update).
type Worker struct {
	q           *jobs.Queue
	downloadDir string
	onFiltered  OnFiltered
	run         RunFunc
	poll        time.Duration
}

// NewWorker crée le worker sur la file donnée. downloadDir est le dossier
// de téléchargement (lecture du set des fichiers sur disque) ; onFiltered
// reçoit chaque résultat et ré-enfile le téléchargement.
func NewWorker(q *jobs.Queue, downloadDir string, onFiltered OnFiltered) *Worker {
	return &Worker{
		q:           q,
		downloadDir: downloadDir,
		onFiltered:  onFiltered,
		run:         Run,
		poll:        jobs.PollInterval,
	}
}

// WithRun remplace la fonction de filtrage (tests : fixtures locales).
func (w *Worker) WithRun(fn RunFunc) *Worker {
	w.run = fn
	return w
}

// Run boucle sur la file : réclame les jobs fast_filter prêts et les
// exécute. Un processus mort en plein filtrage laisse un job 'running' —
// RequeueOrphaned (démarrage) le relance, comme pour les téléchargements.
func (w *Worker) Run() {
	for {
		job, err := w.q.Dequeue("fast_filter")
		if err != nil {
			slog.Error("dequeue fast_filter échoué", "err", err)
			time.Sleep(w.poll)
			continue
		}
		if job == nil {
			time.Sleep(w.poll)
			continue
		}
		w.process(job)
	}
}

// process exécute un job fast_filter réclamé : parse le payload, lance le
// filtre Go, clôture le job, puis appelle onFiltered (application du
// résultat à la tâche + ré-enfilage du téléchargement).
func (w *Worker) process(job *store.Job) {
	var p Payload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil || p.URL == "" || p.TaskID == "" {
		// Payload illisible : on clôture en échec (une tâche orpheline dans
		// l'UI vaut mieux qu'une file bloquée).
		if err := w.q.Complete(job.ID, "", "payload fast_filter invalide"); err != nil {
			slog.Error("job fast_filter non clôturé", "id", job.ID, "err", err)
		}
		return
	}

	res := w.run(w.downloadDir, p.URL, nil)
	if err := w.q.Complete(job.ID, sourceOf(p.URL), ""); err != nil {
		slog.Error("job fast_filter non clôturé", "id", job.ID, "err", err)
		return
	}
	if w.onFiltered != nil {
		w.onFiltered(p, res)
	}
}

// sourceOf extrait l'hôte de l'URL (circuit breaker par source, même
// convention que le worker download).
func sourceOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
