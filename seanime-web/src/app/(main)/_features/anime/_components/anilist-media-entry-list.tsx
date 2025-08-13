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

    // Client-side pagination to reduce render cost
    const entries = React.useMemo(() => list?.entries?.filter(Boolean) || [], [list?.entries])
    const [page, setPage] = React.useState(1)
    const pageSize = 36
    const pageCount = Math.max(1, Math.ceil(entries.length / pageSize))
    const start = (page - 1) * pageSize
    const end = start + pageSize
    const pageEntries = entries.slice(start, end)

    // Reset page when list changes
    React.useEffect(() => {
        setPage(1)
    }, [list?.entries])

    return (
        <div data-anilist-anime-entry-list className="space-y-4">
            <MediaCardLazyGrid itemCount={pageEntries.length}>
                {pageEntries.map((entry) => (
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
