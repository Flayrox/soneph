import React, { useState } from "react";
import { Sparkles, ArrowRight, Check } from "lucide-react";
import { MODULES, defaultModules } from "@/modules";
import { useI18n } from "@/i18n";

interface OnboardingViewProps {
  onFinish: (ids: string[]) => void;
}

export const OnboardingView: React.FC<OnboardingViewProps> = ({ onFinish }) => {
  const { t } = useI18n();
  const [selected, setSelected] = useState<Set<string>>(() => new Set(defaultModules()));

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="fixed inset-0 z-[60] bg-[#161618]/95 backdrop-blur-xl flex items-center justify-center p-6 overflow-y-auto">
      <div className="w-full max-w-lg space-y-6">
        {/* Title */}
        <div className="text-center">
          <div className="inline-flex items-center gap-2 text-apple-pink mb-2">
            <Sparkles className="w-4 h-4" />
            <span className="text-[11px] font-bold uppercase tracking-wider">{t("Welcome to soneph")}</span>
          </div>
          <h2 className="text-2xl font-bold text-white">{t("Choose your modules")}</h2>
          <p className="text-sm text-zinc-400 mt-2">
            {t("Enable the features you want. You can change this anytime in the Marketplace.")}
          </p>
        </div>

        {/* Module cards */}
        <div className="space-y-3">
          {MODULES.map((m) => {
            const isOn = selected.has(m.id);
            return (
              <button
                key={m.id}
                onClick={() => toggle(m.id)}
                className={`w-full text-left bg-white/5 border rounded-2xl p-4 flex items-center gap-4 transition-all ${
                  isOn ? "border-apple-pink/40" : "border-white/10 opacity-70 hover:opacity-100"
                }`}
              >
                <div className="w-11 h-11 rounded-xl bg-apple-pink/15 border border-apple-pink/25 flex items-center justify-center text-apple-pink shrink-0">
                  <m.icon className="w-5 h-5" />
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-bold text-white">{t(m.nameKey)}</h3>
                  <p className="text-xs text-zinc-400 mt-0.5">{t(m.descKey)}</p>
                </div>
                <div
                  className={`w-6 h-6 rounded-full border flex items-center justify-center shrink-0 transition-all ${
                    isOn ? "bg-apple-pink border-apple-pink" : "border-white/25"
                  }`}
                >
                  {isOn && <Check className="w-4 h-4 text-white" />}
                </div>
              </button>
            );
          })}
        </div>

        {/* Continue */}
        <button
          onClick={() => onFinish([...selected])}
          className="w-full flex items-center justify-center gap-2 bg-apple-pink hover:bg-apple-pinkHover text-white font-bold text-sm py-3 rounded-full transition-all shadow-lg active:scale-[0.98]"
        >
          {t("Continue")}
          <ArrowRight className="w-4 h-4" />
        </button>

        <p className="text-center text-[11px] text-zinc-600">
          {t("All modules are optional — the player, library and lyrics stay always on.")}
        </p>
      </div>
    </div>
  );
};
