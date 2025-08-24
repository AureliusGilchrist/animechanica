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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Missing</h1>
        <p className="text-muted-foreground">Owned: {entriesLen.toLocaleString()}</p>
      </div>
      <Separator />
      {isLoading && <p className="text-muted-foreground">Loading library…</p>}
      <div className="space-y-8">
        {visible.map((m) => (
          <MissingItem key={m.id} parent={m} ownedIds={ownedIds} />
        ))}
      </div>
      {/* Sentinel for lazy batch rendering */}
      <div ref={loadMoreRef} className="h-8" />
    </div>
  )
}
