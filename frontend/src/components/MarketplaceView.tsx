import React from "react";
import { Puzzle, Check } from "lucide-react";
import { MODULES, useModules } from "@/modules";
import { useI18n } from "@/i18n";

export const MarketplaceView: React.FC = () => {
  const { t } = useI18n();
  const { enabled, toggle } = useModules();

  return (
    <div className="w-full text-zinc-200 select-none font-sans p-6 space-y-6 max-w-2xl">
      <div className="flex items-center gap-2 text-apple-pink">
        <Puzzle className="w-4 h-4" />
        <span className="text-[11px] font-semibold uppercase tracking-wider">{t("Modules")}</span>
      </div>
      <p className="text-sm text-zinc-400 -mt-3">
        {t("Enable the features you want. You can change this anytime in the Marketplace.")}
      </p>

      <div className="space-y-3">
        {MODULES.map((m) => {
          const isOn = enabled.has(m.id);
          return (
            <div
              key={m.id}
              className={`bg-white/5 border rounded-2xl p-4 flex items-center gap-4 transition-colors ${
                isOn ? "border-apple-pink/30" : "border-white/10 opacity-70"
              }`}
            >
              <div className="w-11 h-11 rounded-xl bg-apple-pink/15 border border-apple-pink/25 flex items-center justify-center text-apple-pink shrink-0">
                <m.icon className="w-5 h-5" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-bold text-white">{t(m.nameKey)}</h3>
                  <span
                    className={`text-[10px] font-bold px-2 py-0.5 rounded-full ${
                      isOn ? "bg-emerald-500/15 text-emerald-400" : "bg-white/10 text-zinc-400"
                    }`}
                  >
                    {isOn ? t("Enabled") : t("Disabled")}
                  </span>
                </div>
                <p className="text-xs text-zinc-400 mt-0.5">{t(m.descKey)}</p>
              </div>

              {/* Toggle switch */}
              <button
                onClick={() => toggle(m.id)}
                className={`w-11 h-6 rounded-full transition-colors relative shrink-0 ${
                  isOn ? "bg-apple-pink" : "bg-white/15"
                }`}
                title={isOn ? t("Disabled") : t("Enabled")}
              >
                <span
                  className={`absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-all ${
                    isOn ? "left-[22px]" : "left-0.5"
                  }`}
                />
              </button>
            </div>
          );
        })}
      </div>

      <div className="flex items-center gap-2 text-[11px] text-zinc-500">
        <Check className="w-3.5 h-3.5 text-emerald-400" />
        {t("Changes apply immediately — no restart needed")}
      </div>
    </div>
  );
};
