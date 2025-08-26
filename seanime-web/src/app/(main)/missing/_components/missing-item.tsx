"use client"
import React from "react"
import { AL_BaseAnime, AL_AnimeDetailsById_Media } from "@/api/generated/types"
import { useGetAnilistAnimeDetails } from "@/api/hooks/anilist.hooks"
import capitalize from "lodash/capitalize"
import { MediaCardGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"

export function MissingItem({ parent, ownedIds }: { parent: AL_BaseAnime; ownedIds: Set<number> }) {
  const rootRef = React.useRef<HTMLDivElement | null>(null)
  const [inView, setInView] = React.useState(false)

  React.useEffect(() => {
    const el = rootRef.current
    if (!el) return
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting) {
            setInView(true)
          }
        })
      },
      { rootMargin: "400px" }
    )
    io.observe(el)
    return () => io.disconnect()
  }, [])

  const { data, isLoading } = useGetAnilistAnimeDetails(inView ? parent.id : undefined)

  const relations = React.useMemo(() => {
    const edges = data?.relations?.edges ?? []
    return edges
      .filter(Boolean)
      .filter(
        (n) =>
          (n.node?.format === "TV" || n.node?.format === "OVA" || n.node?.format === "MOVIE" || n.node?.format === "SPECIAL") &&
          (n.relationType === "PREQUEL" ||
            n.relationType === "SEQUEL" ||
            n.relationType === "PARENT" ||
            n.relationType === "SIDE_STORY" ||
            n.relationType === "ALTERNATIVE")
      )
  }, [data?.relations?.edges])

  const missing = React.useMemo(() => {
    return relations.filter((r) => r.node?.id && !ownedIds.has(r.node.id))
  }, [relations, ownedIds])

  if (!parent) return null

  const title = parent.title?.userPreferred ?? parent.title?.romaji ?? "Untitled"

  return (
    <div ref={rootRef} className="space-y-3 rounded-lg border bg-card/50 p-4">
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-lg md:text-xl font-semibold line-clamp-1" title={title}>{title}</h2>
        <div className="shrink-0 text-xs md:text-sm text-muted-foreground">
          {isLoading ? (
            <span className="inline-flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-amber-500 animate-pulse" /> Loading relations…</span>
          ) : (
            <span>
              <span className="inline-flex items-center rounded-md bg-amber-500/15 text-amber-600 dark:text-amber-400 px-2 py-0.5 font-medium">
                {missing.length}
              </span>{" "}
              missing related
            </span>
          )}
        </div>
      </div>

      {inView && isLoading && (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="relative aspect-[2/3] rounded-md overflow-hidden bg-muted animate-pulse" />
          ))}
        </div>
      )}

      {inView && !isLoading && missing.length === 0 && (
        <div className="text-sm text-muted-foreground">No missing related entries. You're all caught up here.</div>
      )}

      {inView && !isLoading && missing.length > 0 && (
        <MediaCardGrid>
          {missing.map((edge) => (
            <div key={edge.node?.id} className="col-span-1 relative">
              <MediaEntryCard
                media={edge.node!}
                overlay={
                  <p className="font-semibold text-white bg-gray-950 z-[-1] absolute right-0 w-fit px-3 py-1 text-center !bg-opacity-90 text-xs lg:text-sm rounded-none rounded-bl-lg border border-t-0 border-r-0">
                    {edge.node?.format === "MOVIE"
                      ? capitalize(edge.relationType || "").replace(/_/g, " ") + " (Movie)"
                      : capitalize(edge.relationType || "").replace(/_/g, " ")}
                  </p>
                }
                type="anime"
              />
              <span className="absolute left-2 top-2 text-[10px] md:text-xs bg-amber-600/90 text-white px-2 py-0.5 rounded">Missing</span>
            </div>
          ))}
        </MediaCardGrid>
      )}
    </div>
  )
}
