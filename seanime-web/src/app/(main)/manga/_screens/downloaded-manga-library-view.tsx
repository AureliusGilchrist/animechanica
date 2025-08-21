import React from "react"
import { useGetDownloadedMangaSeries, useRefreshDownloadedMangaCache, DownloadedMangaSeries } from "@/api/hooks/manga_downloaded.hooks"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Button } from "@/components/ui/button"
import { LuffyError } from "@/components/shared/luffy-error"
import { MediaCardLazyGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { LuRefreshCw } from "react-icons/lu"
import { toast } from "sonner"
import Link from "next/link"
import { useKitsuPoster } from "@/lib/kitsu/useKitsuPoster"

type DownloadedMangaLibraryViewProps = {
    search?: string
    selectedGenres?: string[]
    mediaGenresById?: Record<number, string[]>
}

export function DownloadedMangaLibraryView(props: DownloadedMangaLibraryViewProps) {
    const { search, selectedGenres = [], mediaGenresById = {} } = props
    const { data: downloadedSeries, isLoading, error, refetch } = useGetDownloadedMangaSeries()
    const refreshCacheMutation = useRefreshDownloadedMangaCache()

    const handleRefreshCache = () => {
        refreshCacheMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("Downloaded manga cache refreshed")
                refetch()
            },
            onError: (error) => {
                toast.error("Failed to refresh cache: " + error.message)
            }
        })
    }

    if (isLoading) {
        return (
            <PageWrapper className="space-y-6">
                <div className="flex items-center justify-center min-h-[50vh]">
                    <LoadingSpinner />
                </div>
            </PageWrapper>
        )
    }

    if (error) {
        return (
            <PageWrapper className="space-y-6">
                <LuffyError title="Failed to load downloaded manga">
                    <p>Could not fetch downloaded manga series. Please try again.</p>
                    <Button onClick={() => refetch()} className="mt-4">
                        Retry
                    </Button>
                </LuffyError>
            </PageWrapper>
        )
    }

    // Apply client-side filtering by title using the provided search term
    const filtered = React.useMemo(() => {
        const list = downloadedSeries || []
        const q = (search || "").trim().toLowerCase()
        const titleFiltered = q ? list.filter((s) => s.seriesTitle?.toLowerCase().includes(q)) : list

        // Apply genre filtering if any selected
        if (!selectedGenres.length) return titleFiltered
        return titleFiltered.filter((s) => {
            if (!s.mediaId) return false
            const g = mediaGenresById[s.mediaId]
            if (!g || !g.length) return false
            // OR match: include if any selected genre is present
            return selectedGenres.some((sel) => g.includes(sel))
        })
    }, [downloadedSeries, search, selectedGenres, mediaGenresById])

    return (
        <PageWrapper className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold">Downloaded Manga</h1>
                    <p className="text-muted-foreground mt-1">
                        {(filtered?.length ?? downloadedSeries?.length ?? 0)} series downloaded
                    </p>
                </div>
                <Button
                    onClick={handleRefreshCache}
                    disabled={refreshCacheMutation.isPending}
                    intent="white-outline"
                    size="sm"
                >
                    <LuRefreshCw className={`mr-2 h-4 w-4 ${refreshCacheMutation.isPending ? 'animate-spin' : ''}`} />
                    Refresh
                </Button>
            </div>

            {!filtered || filtered.length === 0 ? (
                <div className="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
                    <div className="text-center">
                        <h3 className="text-lg font-medium text-muted-foreground">No downloaded manga found</h3>
                        <p className="text-sm text-muted-foreground mt-2">
                            Download some manga chapters to see them here
                        </p>
                    </div>
                </div>
            ) : (
                <MediaCardLazyGrid itemCount={filtered.length}>
                    {filtered.map((series) => (
                        <DownloadedMangaCard
                            key={series.seriesPath}
                            series={series}
                        />
                    ))}
                </MediaCardLazyGrid>
            )}
        </PageWrapper>
    )
}

type DownloadedMangaCardProps = {
    series: DownloadedMangaSeries
}

function DownloadedMangaCard({ series }: DownloadedMangaCardProps) {
    // Create a link to the manga details page
    const detailsLink = series.mediaId 
        ? `/manga/entry?id=${series.mediaId}` 
        : `/manga/local/${encodeURIComponent(series.seriesTitle)}`

    // Prefer Kitsu poster when available, fallback to local cover if present
    const { url: kitsuPoster } = useKitsuPoster(series.seriesTitle)
    const localCover = series.coverImagePath
        ? `/api/v1/manga/local-page/${encodeURIComponent(series.coverImagePath)}`
        : null
    const imgSrc = kitsuPoster || localCover || "/no-cover.png"

    return (
        <Link href={detailsLink}>
            <div className="group relative overflow-hidden rounded-lg border bg-card hover:bg-accent/50 transition-colors cursor-pointer">
                {/* Cover Image */}
                <div className="aspect-[3/4] relative overflow-hidden bg-muted">
                    {imgSrc ? (
                        <img
                            src={imgSrc}
                            alt={series.seriesTitle}
                            className="object-cover w-full h-full group-hover:scale-105 transition-transform duration-300"
                            loading="lazy"
                        />
                    ) : (
                        <div className="flex items-center justify-center w-full h-full bg-muted">
                            <span className="text-4xl text-muted-foreground">📚</span>
                        </div>
                    )}
                    
                    {/* Chapter Count Badge */}
                    <div className="absolute top-2 right-2 bg-primary text-primary-foreground text-xs px-2 py-1 rounded-full">
                        {series.chapterCount} chapters
                    </div>
                </div>

                {/* Series Info */}
                <div className="p-3">
                    <h3 className="font-medium text-sm line-clamp-2 mb-1">
                        {series.seriesTitle}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                        Last updated: {new Date(series.lastUpdated * 1000).toLocaleDateString()}
                    </p>
                </div>
            </div>
        </Link>
    )
}
