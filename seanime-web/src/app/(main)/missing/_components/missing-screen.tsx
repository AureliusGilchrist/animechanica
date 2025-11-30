"use client"
import React from "react"
import { useGetLibraryCollection } from "@/api/hooks/anime_collection.hooks"
import { AL_BaseAnime } from "@/api/generated/types"
import { Separator } from "@/components/ui/separator"
import { MissingItem } from "./missing-item"

export function MissingScreen() {
  const { data, isLoading } = useGetLibraryCollection()

  const entries: Array<AL_BaseAnime> = React.useMemo(() => {
    const lists = data?.lists ?? []
    const items: Array<AL_BaseAnime> = []
    lists?.forEach((l) => {
      l?.entries?.forEach((e) => {
        if (e?.media) items.push(e.media)
      })
    })
    return items
  }, [data?.lists])

  const ownedIds = React.useMemo(() => new Set<number>(entries.map((m) => m.id).filter(Boolean)), [entries])

  // Lazy rendering by batches to avoid UI lag
  const [visibleCount, setVisibleCount] = React.useState(20)
  const loadMoreRef = React.useRef<HTMLDivElement | null>(null)
  const entriesLen = entries.length

  React.useEffect(() => {
    const el = loadMoreRef.current
    if (!el) return
    const io = new IntersectionObserver((obsEntries) => {
      obsEntries.forEach((en) => {
        if (en.isIntersecting) {
          setVisibleCount((c) => Math.min(c + 20, entriesLen))
        }
      })
    })
    io.observe(el)
    return () => io.disconnect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadMoreRef.current])
  const visible = entries.slice(0, visibleCount)

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Missing</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Related anime you don’t own yet. Browse per series and fill the gaps.
          </p>
        </div>
        <div className="shrink-0 self-center text-sm text-muted-foreground">
          <span className="mr-2">Owned</span>
          <span className="inline-flex items-center rounded-md bg-primary/10 text-primary px-2 py-0.5 font-medium">
            {entriesLen.toLocaleString()}
          </span>
        </div>
      </div>
      <Separator />

      {isLoading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="rounded-lg border bg-card/50 p-4">
              <div className="flex items-center justify-between">
                <div className="h-5 w-40 bg-muted rounded animate-pulse" />
                <div className="h-4 w-24 bg-muted rounded animate-pulse" />
              </div>
              <div className="mt-4 grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
                {Array.from({ length: 6 }).map((__, j) => (
                  <div key={j} className="relative aspect-[2/3] rounded-md overflow-hidden bg-muted animate-pulse" />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {!isLoading && entriesLen === 0 && (
        <div className="rounded-lg border bg-card/50 p-8 text-center">
          <h2 className="text-lg font-medium">No entries in your library</h2>
          <p className="text-sm text-muted-foreground mt-1">Add anime to your library to see missing related entries here.</p>
        </div>
      )}

      {!isLoading && entriesLen > 0 && (
        <div className="space-y-6">
          {visible.map((m) => (
            <MissingItem key={m.id} parent={m} ownedIds={ownedIds} />
          ))}
        </div>
      )}

      {/* Sentinel for lazy batch rendering */}
      <div ref={loadMoreRef} className="h-10" />
    </div>
  )
}
