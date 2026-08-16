import React, { useEffect, useRef, useState } from "react";
import { X, Disc, Music2 } from "lucide-react";
import type { DownloadedFile } from "@/types";
import { useI18n } from "@/i18n";

interface LyricsModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentTrack: DownloadedFile | null;
  currentTime: number;
  lyricsRaw: string | null;
}

interface LyricLine {
  time: number;
  text: string;
}

export const LyricsModal: React.FC<LyricsModalProps> = ({
  isOpen,
  onClose,
  currentTrack,
  currentTime,
  lyricsRaw,
}) => {
  const { t } = useI18n();
  const [parsedLines, setParsedLines] = useState<LyricLine[]>([]);
  const activeLineRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!lyricsRaw) {
      setParsedLines([]);
      return;
    }

    const lines = lyricsRaw.split("\n");
    const parsed: LyricLine[] = [];
    const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;

    lines.forEach((line) => {
      const match = timeRegex.exec(line);
      if (match) {
        const min = parseInt(match[1], 10);
        const sec = parseInt(match[2], 10);
        const ms = parseInt(match[3], 10);
        const totalTime = min * 60 + sec + (ms > 99 ? ms / 1000 : ms / 100);
        const text = line.replace(timeRegex, "").trim();
        if (text) {
          parsed.push({ time: totalTime, text });
        }
      }
    });

    setParsedLines(parsed);
  }, [lyricsRaw]);

  // Find active line index based on current playback time
  let activeIndex = -1;
  for (let i = 0; i < parsedLines.length; i++) {
    if (currentTime >= parsedLines[i].time) {
      activeIndex = i;
    } else {
      break;
    }
  }

  // Scroll active line into center view smoothly
  useEffect(() => {
    if (activeLineRef.current) {
      activeLineRef.current.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    }
  }, [activeIndex]);

  if (!isOpen || !currentTrack) return null;

  return (
    <div className="fixed inset-0 z-50 bg-black/95 backdrop-blur-2xl flex flex-col justify-between p-6 sm:p-12 animate-fade-in select-none">
      {/* Top Controls */}
      <div className="flex items-center justify-between border-b border-white/10 pb-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded bg-[#28282c] flex items-center justify-center text-apple-subtext">
            <Disc className="w-6 h-6" />
          </div>
          <div>
            <h3 className="text-base font-bold text-white truncate max-w-sm">{currentTrack.title}</h3>
            <p className="text-xs text-apple-subtext">{currentTrack.artist} • {t("Synced Lyrics")}</p>
          </div>
        </div>

        <button
          onClick={onClose}
          className="w-9 h-9 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition-colors"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Main Synchronized Lyrics Stream */}
      <div className="flex-1 overflow-y-auto my-8 px-4 sm:px-16 scrollbar-none flex flex-col items-start justify-start space-y-6">
        {parsedLines.length === 0 ? (
          <div className="m-auto text-center text-apple-subtext">
            <Music2 className="w-12 h-12 mx-auto mb-3 opacity-30" />
            <p className="text-base font-semibold text-white">{t("No Synced Lyrics File Available")}</p>
            <p className="text-xs mt-1">{t("This song was downloaded without a .LRC synced lyrics file.")}</p>
          </div>
        ) : (
          parsedLines.map((line, idx) => {
            const isActive = idx === activeIndex;
            const isPast = idx < activeIndex;

            return (
              <div
                key={idx}
                ref={isActive ? activeLineRef : null}
                className={`text-2xl sm:text-4xl md:text-5xl font-black transition-all duration-300 transform origin-left cursor-pointer ${
                  isActive
                    ? "text-emerald-400 scale-105 drop-shadow-[0_0_15px_rgba(16,185,129,0.4)] opacity-100"
                    : isPast
                    ? "text-white/40 scale-100"
                    : "text-white/20 scale-100"
                }`}
              >
                {line.text}
              </div>
            );
          })
        )}
      </div>

      {/* Footer hint */}
      <div className="text-center text-xs text-apple-subtext border-t border-white/10 pt-4">
        <span>{t("Press ESC or click close to return to your library")}</span>
      </div>
    </div>
  );
};
