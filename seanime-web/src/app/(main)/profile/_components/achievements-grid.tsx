"use client"

import React from "react"
import type { Achievement } from "../_lib/achievements"

export function AchievementsGrid({ items }: { items: Achievement[] }) {
  const [category, setCategory] = React.useState<string>("All")

  const categories = React.useMemo(() => {
    const set = new Set<string>(["All"]) 
    for (const a of items) set.add(a.category)
    return Array.from(set)
  }, [items])

  const filtered = React.useMemo(() => {
    return category === "All" ? items : items.filter(a => a.category === category)
  }, [category, items])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        {categories.map(cat => (
          <button
            key={cat}
            onClick={() => setCategory(cat)}
            className={`rounded-full border px-3 py-1 text-xs ${category === cat ? "bg-background" : "bg-card/60"}`}
          >
            {cat}
          </button>
        ))}
      </div>
      <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {filtered.map(a => (
          <AchievementCard key={a.id} a={a} />
        ))}
      </div>
    </div>
  )
}

function AchievementCard({ a }: { a: Achievement }) {
  const pct = Math.min(100, Math.round((a.progress / a.target) * 100))
  return (
    <div className={`relative overflow-hidden rounded-xl border p-4 bg-card/60 ${a.unlocked ? "ring-1 ring-emerald-500/30" : ""}`}>
      <div className="flex items-start gap-3">
        <div className={`grid h-10 w-10 place-items-center rounded-lg border text-lg ${a.unlocked ? "bg-emerald-500/10" : "bg-muted/30"}`}>
          <span aria-hidden>{a.icon}</span>
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h4 className="font-semibold truncate">{a.name}</h4>
            {a.unlocked && <span className="rounded-full border bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 px-2 py-0.5 text-[10px]">Unlocked</span>}
          </div>
          <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">{a.description}</p>
          <div className="mt-3">
            <div className="flex items-center justify-between text-[11px] text-muted-foreground mb-1">
              <span>{a.progress.toLocaleString()} / {a.target.toLocaleString()}</span>
              <span>{pct}%</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div className={`h-full ${a.unlocked ? "bg-emerald-500" : "bg-primary"}`} style={{ width: `${pct}%` }} />
            </div>
          </div>
        </div>
      </div>
      {!a.unlocked && (
        <div className="pointer-events-none absolute -right-16 -top-16 h-32 w-32 rounded-full bg-primary/5 blur-2xl" />
      )}
    </div>
  )
}
