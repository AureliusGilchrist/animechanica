import { AL_AnimeCollection_MediaListCollection_Lists } from "@/api/generated/types"
import { MediaCardLazyGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import React from "react"


type AnilistAnimeEntryListProps = {
    list: AL_AnimeCollection_MediaListCollection_Lists | undefined
    type: "anime" | "manga"
}

/**
 * Displays a list of media entry card from an Anilist media list collection.
 */
export function AnilistAnimeEntryList(props: AnilistAnimeEntryListProps) {

    const {
        list,
        type,
        ...rest
    } = props

    // Normalize series title (remove season/cour markers, numbers, punctuation noise)
    const normalizeSeriesTitle = React.useCallback((t?: string | null) => {
        if (!t) return ""
        let s = t
        // Remove common season/cour tags
        s = s.replace(/\b(season|cour|part)\s*\d+\b/gi, "")
        // Remove roman numerals or trailing numbers
        s = s.replace(/\b([ivxlcdm]+)\b/gi, "")
        s = s.replace(/\d+/g, "")
        // Collapse extra spaces and trim
        s = s.replace(/\s{2,}/g, " ").trim()
        return s.toLowerCase()
    }, [])

    // Determine a chronological key for entries within the same series
    const chronologicalKey = React.useCallback((entry: NonNullable<NonNullable<typeof list>['entries']>[number]) => {
        const m = entry?.media
        // Prefer start date
        const y = m?.startDate?.year ?? 9999
        const mo = m?.startDate?.month ?? 12
        const d = m?.startDate?.day ?? 31
        // Fallbacks: episodes count then id
        const ep = (m as any)?.episodes ?? 99999
        const id = m?.id ?? 99999999
        return [y, mo, d, ep, id]
    }, [list])

    // Group by normalized series title alphabetically; within each group sort chronologically
    const sortedEntries = React.useMemo(() => {
        const raw = (list?.entries || []).filter(Boolean)
        if (!raw.length) return [] as NonNullable<typeof list>['entries']

        const groups = new Map<string, typeof raw>()
        for (const e of raw) {
            const title = e?.media?.title?.userPreferred || e?.media?.title?.romaji || e?.media?.title?.english || ""
            const key = normalizeSeriesTitle(title)
            const arr = groups.get(key) || []
            arr.push(e!)
            groups.set(key, arr)
        }

        const groupKeys = Array.from(groups.keys()).sort((a, b) => a.localeCompare(b))
        const result: typeof raw = []
        for (const k of groupKeys) {
            const arr = groups.get(k)!
            arr.sort((a, b) => {
                const ak = chronologicalKey(a)
                const bk = chronologicalKey(b)
                for (let i = 0; i < ak.length; i++) {
                    if (ak[i] !== bk[i]) return (ak[i] as number) - (bk[i] as number)
                }
                return 0
            })
            result.push(...arr)
        }
        return result
    }, [list?.entries, normalizeSeriesTitle, chronologicalKey])

    // Client-side pagination to reduce render cost (after sorting)
    const [reverseOrder, setReverseOrder] = React.useState(true)

    // Apply reverse only when toggled on
    const displayEntries = React.useMemo(() => {
        const base = (sortedEntries ?? []) as NonNullable<NonNullable<typeof list>["entries"]>
        return reverseOrder ? base.slice().reverse() : base
    }, [sortedEntries, reverseOrder])
    const [page, setPage] = React.useState(1)
    const pageSize = 36
    const pageCount = Math.max(1, Math.ceil(displayEntries.length / pageSize))
    const start = (page - 1) * pageSize
    const end = start + pageSize
    const pageEntries = displayEntries.slice(start, end)

    // Map of mediaId -> folder exists (for blue badge)
    const [folderExists, setFolderExists] = React.useState<Record<number, boolean>>({})

    // Fetch folder existence for currently visible page entries
    React.useEffect(() => {
        const ids = (pageEntries || []).map(e => e?.media?.id).filter(Boolean) as number[]
        if (!ids.length) {
            setFolderExists({})
            return
        }
        let cancelled = false
        ;(async () => {
            try {
                const results = await Promise.all(ids.map(async (id) => {
                    try {
                        const res = await fetch(`/api/v1/library/anime-entry/dir-exists/${id}`)
                        if (!res.ok) return [id, false] as const
                        const json: any = await res.json()
                        const exists = !!(json?.data?.exists)
                        return [id, exists] as const
                    } catch (_) {
                        return [id, false] as const
                    }
                }))
                if (!cancelled) {
                    const map: Record<number, boolean> = {}
                    for (const [id, exists] of results) map[id] = exists
                    setFolderExists(map)
                }
            } catch (_) {
                if (!cancelled) setFolderExists({})
            }
        })()
        return () => { cancelled = true }
    }, [pageEntries])

    // Reset page when list changes
    React.useEffect(() => {
        setPage(1)
    }, [list?.entries])

    // Reset page when order changes
    React.useEffect(() => {
        setPage(1)
    }, [reverseOrder])

    return (
        <div data-anilist-anime-entry-list className="space-y-4">
            <div className="flex items-center justify-between gap-2">
                <div className="text-sm text-[--muted]">Order: {reverseOrder ? "Z→A / Newest→Oldest" : "A→Z / Oldest→Newest"}</div>
                <button
                    className="px-3 py-1 rounded border"
                    onClick={() => setReverseOrder(v => !v)}
                >
                    Reverse order
                </button>
            </div>
            <MediaCardLazyGrid itemCount={pageEntries.length}>
                {pageEntries.filter(Boolean).map((entry) => (
                <MediaEntryCard
                    key={`${entry.media?.id}`}
                    listData={{
                        progress: entry.progress!,
                        score: entry.score!,
                        status: entry.status!,
                        startedAt: entry.startedAt?.year ? new Date(entry.startedAt.year,
                            (entry.startedAt.month || 1) - 1,
                            entry.startedAt.day || 1).toISOString() : undefined,
                        completedAt: entry.completedAt?.year ? new Date(entry.completedAt.year,
                            (entry.completedAt.month || 1) - 1,
                            entry.completedAt.day || 1).toISOString() : undefined,
                    }}
                    showLibraryBadge
                    media={entry.media!}
                    existingFolder={folderExists[entry.media!.id as number]}
                    showListDataButton
                    type={type}
                />
                ))}
            </MediaCardLazyGrid>

            {pageCount > 1 && (
                <div className="flex items-center justify-center gap-3 pt-2">
                    <button
                        className="px-3 py-1 rounded border disabled:opacity-50"
                        onClick={() => setPage(p => Math.max(1, p - 1))}
                        disabled={page === 1}
                    >
                        Prev
                    </button>
                    <span className="text-sm text-[--muted]">
                        Page {page} / {pageCount}
                    </span>
                    <button
                        className="px-3 py-1 rounded border disabled:opacity-50"
                        onClick={() => setPage(p => Math.min(pageCount, p + 1))}
                        disabled={page === pageCount}
                    >
                        Next
                    </button>
                </div>
            )}
        </div>
    )
}
