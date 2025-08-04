import React from "react"
import { AL_MangaCollection_MediaListCollection_Lists_Entries } from "@/api/generated/types"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { cn } from "@/components/ui/core/styling"
import { Skeleton } from "@/components/ui/skeleton"
import { useThemeSettings } from "@/lib/theme/hooks"

type MangaContinueReadingProps = {
    entries: AL_MangaCollection_MediaListCollection_Lists_Entries[]
    isLoading: boolean
}

export function MangaContinueReading({ entries, isLoading }: MangaContinueReadingProps) {
    const ts = useThemeSettings()

    if (isLoading) {
        return (
            <PageWrapper className="p-4 space-y-4 relative z-[4]" data-manga-continue-reading-container>
                <Skeleton className="h-8 w-48" />
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-7 min-[2000px]:grid-cols-8 gap-4">
                    {[1, 2, 3, 4, 5, 6, 7, 8]?.map((_, idx) => (
                        <Skeleton
                            key={idx}
                            className={cn(
                                "h-[22rem] min-[2000px]:h-[24rem] col-span-1 aspect-[6/7] flex-none rounded-[--radius-md] relative overflow-hidden",
                                "[&:nth-child(8)]:hidden min-[2000px]:[&:nth-child(8)]:block",
                                "[&:nth-child(7)]:hidden 2xl:[&:nth-child(7)]:block",
                                "[&:nth-child(6)]:hidden xl:[&:nth-child(6)]:block",
                                "[&:nth-child(5)]:hidden xl:[&:nth-child(5)]:block",
                                "[&:nth-child(4)]:hidden lg:[&:nth-child(4)]:block",
                                "[&:nth-child(3)]:hidden md:[&:nth-child(3)]:block",
                            )}
                        />
                    ))}
                </div>
            </PageWrapper>
        )
    }

    if (!entries?.length) return null

    return (
        <PageWrapper className="p-4 space-y-4 relative z-[4]" data-manga-continue-reading-container>
            <h2 className="text-lg font-semibold">Continue Reading</h2>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-7 min-[2000px]:grid-cols-8 gap-4">
                {entries.slice(0, 8).map((entry) => {
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
                            key={entry.media.id}
                            media={entry.media}
                            listData={listData}
                            showListDataButton={false}
                            withAudienceScore={false}
                            type="manga"
                        />
                    )
                })}
            </div>
        </PageWrapper>
    )
}
