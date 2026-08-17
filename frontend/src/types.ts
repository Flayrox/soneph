// Types partagés entre les composants — source unique de vérité.

export interface DownloadedFile {
  rel_path: string;
  file_name?: string;
  title: string;
  artist: string;
  album: string;
  duration?: number;
  /** Taille en octets (champ `size` du backend). */
  size?: number;
  size_bytes?: number;
  has_lyrics?: boolean;
  lyrics_type?: "synced" | "unsynced" | "none";
  lrc_path?: string;
  mod_time: string;
}

export interface DownloadTask {
  id: string;
  url: string;
  bitrate?: string;
  order?: string;
  status: "queued" | "downloading" | "completed" | "failed";
  progress: string;
  current_track?: string;
  total_tracks?: number;
  completed_count?: number;
  recent_tracks?: string[];
  logs: string[];
  created_at: string;
  error?: string;
}

/** Ligne de la table jobs (file M4) — panneau « jobs » du DownloadsView. */
export interface JobRow {
  id: string;
  type: string;
  payload: string;
  status: "queued" | "running" | "done" | "failed";
  priority: number;
  attempts: number;
  max_attempts: number;
  error?: string;
  created_at: string;
  started_at?: string | null;
  finished_at?: string | null;
  /** Prochaine échéance d'un retry (backoff M4) — compte à rebours. */
  retry_at?: string | null;
}

export interface PlaylistSummary {
  id: string;
  name: string;
  track_count: number;
  created_at: string;
  updated_at: string;
}

export interface Playlist {
  id: string;
  name: string;
  tracks: DownloadedFile[];
}

export interface HistoryRecord {
  path: string;
  played_at: string;
}

export interface TopTrack {
  path: string;
  plays: number;
}

/** Résultat de GET /api/search — forme JSON d'une ligne de la table tracks. */
export interface SearchTrack {
  id: number;
  path: string;
  title: string;
  artist?: string;
  album?: string;
  track_no?: number;
  duration_ms?: number;
  bitrate?: number;
  format?: string;
  size_bytes?: number;
  isrc?: string;
  lyrics_path?: string;
  lyrics_synced?: boolean;
  quality_score?: number;
  added_at: string;
  updated_at: string;
}
