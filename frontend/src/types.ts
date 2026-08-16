// Types partagés entre les composants — source unique de vérité.

export interface DownloadedFile {
  rel_path: string;
  file_name?: string;
  title: string;
  artist: string;
  album: string;
  duration?: number;
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
