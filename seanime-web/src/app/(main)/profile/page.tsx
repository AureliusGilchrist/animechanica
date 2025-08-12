"use client"

import { CustomLibraryBanner } from "@/app/(main)/(library)/_containers/custom-library-banner"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { StaticTabs } from "@/components/ui/tabs"
import React from "react"
import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"
import { useGetRawAnimeCollection } from "@/api/hooks/anilist.hooks"
import { useGetRawAnilistMangaCollection } from "@/api/hooks/manga.hooks"
import { AL_AnimeCollection_MediaListCollection_Lists } from "@/api/generated/types"
import { AnilistAnimeEntryList } from "@/app/(main)/_features/anime/_components/anilist-media-entry-list"
import { ProfileBanner } from "./_components/profile-banner"
import { AchievementsGrid } from "./_components/achievements-grid"
import { computeAchievements } from "./_lib/achievements"
import { OnlineUsersSidebar } from "./_components/online-users-sidebar"

export const dynamic = "force-static"

export default function ProfilePage() {
    const serverStatus = useServerStatus()
    const [tab, setTab] = React.useState<"anime" | "manga">("anime")

    const { data: animeData, isLoading: isAnimeLoading } = useGetRawAnimeCollection()
    const { data: mangaData, isLoading: isMangaLoading } = useGetRawAnilistMangaCollection()

    const enableManga = !!serverStatus?.settings?.library?.enableManga

    React.useEffect(() => {
        if (tab === "manga" && !enableManga) setTab("anime")
    }, [enableManga, tab])

    const completedAnimeList: AL_AnimeCollection_MediaListCollection_Lists | undefined = React.useMemo(() => {
        const lists = animeData?.MediaListCollection?.lists || []
        const completed = lists?.find(l => l?.status === "COMPLETED")
        if (!completed) return undefined
        return {
            name: completed.name,
            isCustomList: completed.isCustomList,
            status: completed.status,
            entries: completed.entries?.filter(Boolean),
        }
    }, [animeData])

    const completedMangaList: AL_AnimeCollection_MediaListCollection_Lists | undefined = React.useMemo(() => {
        const lists = mangaData?.MediaListCollection?.lists || []
        const completed = lists?.find(l => l?.status === "COMPLETED")
        if (!completed) return undefined
        return {
            name: completed.name,
            isCustomList: completed.isCustomList,
            status: completed.status,
            entries: completed.entries?.filter(Boolean),
        }
    }, [mangaData])

    const showList = tab === "anime" ? completedAnimeList : completedMangaList
    const isLoading = tab === "anime" ? isAnimeLoading : isMangaLoading

    const animeCount = completedAnimeList?.entries?.length ?? 0
    const mangaCount = completedMangaList?.entries?.length ?? 0

    // Derive AniList userId if present in collection payload (optional)
    const userId: number | undefined = (animeData as any)?.MediaListCollection?.user?.id
        ?? (mangaData as any)?.MediaListCollection?.user?.id

    const [meta, setMeta] = React.useState<{ hasBanner: boolean; hasBio: boolean }>({ hasBanner: false, hasBio: false })

    const achievements = React.useMemo(() => computeAchievements({
        anime: animeData?.MediaListCollection,
        manga: mangaData?.MediaListCollection,
    }, { userId, hasBanner: meta.hasBanner, hasBio: meta.hasBio }), [animeData, mangaData, userId, meta])

    return (
        <>
            <CustomLibraryBanner discrete />
            <PageWrapper
                className="p-4 sm:p-8 pt-4 relative"
                data-anilist-profile-page
                {...{
                    initial: { opacity: 0, y: 10 },
                    animate: { opacity: 1, y: 0 },
                    exit: { opacity: 0, y: 10 },
                    transition: { type: "spring", damping: 20, stiffness: 100 },
                }}
            >
                <div className="grid grid-cols-1 xl:grid-cols-[1fr_320px] gap-6">
                    <AppLayoutStack className="space-y-6" data-anilist-profile-stack>
                        {/* Hero Header */}
                        <div className="relative overflow-hidden rounded-2xl border bg-gradient-to-br from-[--card]/60 to-transparent backdrop-blur supports-[backdrop-filter]:backdrop-blur-lg">
                            <div className="p-6 sm:p-8">
                                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                                    <div>
                                        <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight">AniList Profile</h1>
                                        <p className="text-sm text-muted-foreground mt-1">Your completed collections, beautifully presented.</p>
                                    </div>
                                    {/* Stat Pills */}
                                    <div className="flex items-center gap-2 sm:gap-3">
                                        <StatPill label="Completed Anime" value={animeCount} accent="from-indigo-500/20 to-indigo-500/10" />
                                        {enableManga && (
                                            <StatPill label="Completed Manga" value={mangaCount} accent="from-emerald-500/20 to-emerald-500/10" />
                                        )}
                                    </div>
                                </div>
                                {/* Sticky Tabs */}
                                <div className="mt-4">
                                    <div className="w-full flex justify-center">
                                        <StaticTabs
                                            className="h-10 w-fit border rounded-full bg-background/60 backdrop-blur"
                                            triggerClass="px-4 py-1"
                                            items={[
                                                { name: "Anime", isCurrent: tab === "anime", onClick: () => setTab("anime") },
                                                ...(enableManga ? [{ name: "Manga", isCurrent: tab === "manga", onClick: () => setTab("manga") }] : []),
                                            ]}
                                        />
                                    </div>
                                </div>
                            </div>
                        </div>
                        {/* Profile Banner + Bio */}
                        <ProfileBanner userId={userId} onMetaChange={setMeta} />

                        {/* Achievements */}
                        <div className="rounded-2xl border bg-card/60 backdrop-blur supports-[backdrop-filter]:backdrop-blur-lg p-4 sm:p-6">
                            <div className="flex items-center justify-between mb-3">
                                <h2 className="text-base sm:text-lg font-semibold">Achievements <span className="text-[--muted] font-medium ml-2">{achievements.length}</span></h2>
                            </div>
                            <AchievementsGrid items={achievements} />
                        </div>

                        {/* Completed Content Card */}
                        <div className="rounded-2xl border bg-card/60 backdrop-blur supports-[backdrop-filter]:backdrop-blur-lg p-4 sm:p-6">
                            <div className="flex items-center justify-between mb-3">
                                <h2 className="text-base sm:text-lg font-semibold">
                                    Completed {tab === "anime" ? "Anime" : "Manga"}
                                    <span className="text-[--muted] font-medium ml-2">{(showList?.entries?.length ?? 0).toLocaleString()}</span>
                                </h2>
                            </div>

                            {isLoading ? (
                                <GridSkeleton />
                            ) : showList && (showList.entries?.length ?? 0) > 0 ? (
                                <AnilistAnimeEntryList type={tab} list={showList} />
                            ) : (
                                <EmptyState tab={tab} />
                            )}
                        </div>
                    </AppLayoutStack>
                    {/* Right sidebar: Online users (desktop) */}
                    <div className="hidden xl:block">
                        <OnlineUsersSidebar className="top-6" />
                    </div>
                </div>
            </PageWrapper>
        </>
    )
}

function StatPill({ label, value, accent }: { label: string; value: number; accent?: string }) {
    return (
        <div className={`rounded-full border bg-gradient-to-br ${accent ?? "from-white/5 to-white/0"} px-3 py-1.5`}> 
            <div className="flex items-baseline gap-2">
                <span className="text-xs text-muted-foreground">{label}</span>
                <span className="text-sm font-semibold">{value.toLocaleString()}</span>
            </div>
        </div>
    )
}

function GridSkeleton() {
    return (
        <div className="grid gap-3 sm:gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {Array.from({ length: 12 }).map((_, i) => (
                <div key={i} className="aspect-[2/3] rounded-xl border bg-card animate-pulse" />
            ))}
        </div>
    )
}

function EmptyState({ tab }: { tab: "anime" | "manga" }) {
    return (
        <div className="flex flex-col items-center justify-center text-center py-16">
            <div className="inline-flex h-16 w-16 items-center justify-center rounded-full border bg-card mb-4">
                <div className="h-8 w-8 rounded-md bg-gradient-to-br from-white/10 to-white/0" />
            </div>
            <h3 className="text-lg font-semibold">No completed {tab} yet</h3>
            <p className="text-sm text-muted-foreground mt-1 max-w-md">
                Start tracking your progress on AniList and your completed titles will appear here automatically.
            </p>
        </div>
    )
}
