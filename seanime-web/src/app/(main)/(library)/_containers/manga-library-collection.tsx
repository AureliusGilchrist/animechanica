import React from "react"
import { AL_MangaCollection_MediaListCollection_Lists, AL_MangaCollection_MediaListCollection_Lists_Entries } from "@/api/generated/types"
import { __mangaLibrary_paramsAtom } from "@/app/(main)/(library)/_lib/manga-library-params"
import { MediaCardLazyGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { IconButton } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { useAtom } from "jotai/react"
import { LuListFilter } from "react-icons/lu"

const STATUS_LABELS: Record<string, string> = {
    CURRENT: "Reading",
    PLANNING: "Planning",
    COMPLETED: "Completed",
    DROPPED: "Dropped",
    PAUSED: "Paused",
    REPEATING: "Repeating",
    DOWNLOADED: "Downloaded",
}

export function MangaLibraryCollectionLists({ collectionList, isLoading }: {
    collectionList: AL_MangaCollection_MediaListCollection_Lists[],
    isLoading: boolean
}) {
    return (
        <PageWrapper
            key="manga-library-collection-lists"
            className="space-y-8"
            data-manga-library-collection-lists
            {...{
                initial: { opacity: 0, y: 60 },
                animate: { opacity: 1, y: 0 },
                exit: { opacity: 0, scale: 0.99 },
                transition: {
                    duration: 0.25,
                },
            }}>
            {collectionList.map(collection => {
                if (!collection.entries?.length) return null
                return <MangaLibraryCollectionListItem key={collection.status} list={collection} />
            })}
        </PageWrapper>
    )
}

export function MangaLibraryCollectionFilteredLists({ collectionList, isLoading }: {
    collectionList: AL_MangaCollection_MediaListCollection_Lists[],
    isLoading: boolean
}) {
    return (
        <PageWrapper
            key="manga-library-collection-filtered-lists"
            className="space-y-8"
            data-manga-library-collection-filtered-lists
            {...{
                initial: { opacity: 0, y: 60 },
                animate: { opacity: 1, y: 0 },
                exit: { opacity: 0, scale: 0.99 },
                transition: {
                    duration: 0.25,
                },
            }}>
            {collectionList.map(collection => {
                if (!collection.entries?.length) return null
                return <MangaLibraryCollectionListItem key={collection.status} list={collection} />
            })}
        </PageWrapper>
    )
}

export const MangaLibraryCollectionListItem = React.memo(({ list }: {
    list: AL_MangaCollection_MediaListCollection_Lists
}) => {
    const isCurrentlyReading = list.status === "CURRENT"
    const [params, setParams] = useAtom(__mangaLibrary_paramsAtom)

    return (
        <React.Fragment key={list.status}>
            <div className="flex gap-3 items-center" data-manga-library-collection-list-item-header data-list-type={list.status}>
                <h2 className="p-0 m-0">{STATUS_LABELS[list.status || ""] || list.name || "Unknown"}</h2>
                <div className="flex flex-1"></div>
                {isCurrentlyReading && <DropdownMenu
                    trigger={<IconButton
                        intent="white-basic"
                        size="xs"
                        className="mt-1"
                        icon={<LuListFilter />}
                    />}
                >
                    <DropdownMenuItem
                        onClick={() => {
                            setParams(draft => {
                                draft.continueReadingOnly = !draft.continueReadingOnly
                                return
                            })
                        }}
                    >
                        {params.continueReadingOnly ? "Show all" : "Show unread only"}
                    </DropdownMenuItem>
                </DropdownMenu>}
            </div>
            <MediaCardLazyGrid
                itemCount={list?.entries?.length || 0}
                data-manga-library-collection-list-item-media-card-lazy-grid
                data-list-type={list.status}
            >
                {list.entries?.filter(entry => entry.media != null).map(entry => {
                    return <MangaLibraryCollectionEntryItem key={entry.id} entry={entry} />
                })}
            </MediaCardLazyGrid>
        </React.Fragment>
    )
})

export const MangaLibraryCollectionEntryItem = React.memo(({ entry }: {
    entry: AL_MangaCollection_MediaListCollection_Lists_Entries
}) => {
    // Return null if media is not available
    if (!entry.media) return null
    
    // Create a listData object from the entry properties
    const listData = {
        status: entry.status,
        score: entry.score,
        progress: entry.progress,
        repeat: entry.repeat,
        startedAt: entry.startedAt && entry.startedAt.year && entry.startedAt.month && entry.startedAt.day 
            ? `${entry.startedAt.year}-${entry.startedAt.month}-${entry.startedAt.day}` 
            : undefined,
        completedAt: entry.completedAt && entry.completedAt.year && entry.completedAt.month && entry.completedAt.day 
            ? `${entry.completedAt.year}-${entry.completedAt.month}-${entry.completedAt.day}` 
            : undefined,
    }

    return (
        <MediaEntryCard
            media={entry.media!}
            listData={listData}
            showListDataButton
            withAudienceScore={false}
            type="manga"
        />
    )
})
