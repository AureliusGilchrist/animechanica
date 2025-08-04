import React from "react"
import { useGetMangaDownloadsList } from "@/api/hooks/manga_download.hooks"
import { AL_MangaCollection_MediaListCollection_Lists, AL_MediaListStatus } from "@/api/generated/types"
import { MangaLibraryCollectionLists, MangaLibraryCollectionFilteredLists } from "@/app/(main)/(library)/_containers/manga-library-collection"
import { MangaGenreSelector } from "@/app/(main)/(library)/_containers/manga-genre-selector"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { cn } from "@/components/ui/core/styling"
import { Skeleton } from "@/components/ui/skeleton"
import { useThemeSettings } from "@/lib/theme/hooks"
import { useAtom } from "jotai/react"
import { AnimatePresence } from "motion/react"
import { __mangaLibrary_paramsAtom } from "@/app/(main)/(library)/_lib/manga-library-params"

type MangaLibraryViewProps = {
    // Props will be passed from the main library page
}

export function MangaLibraryView(props: MangaLibraryViewProps) {
    const { data: downloadedManga, isLoading: isDownloadedLoading } = useGetMangaDownloadsList()
    
    const ts = useThemeSettings()
    const [params] = useAtom(__mangaLibrary_paramsAtom)
    
    const isLoading = isDownloadedLoading
    
    // Process downloaded manga data only
    const downloadedMangaList = downloadedManga || []
    
    // Extract genres from downloaded manga only
    const allGenres = React.useMemo(() => {
        const genreSet = new Set<string>()
        
        // Add genres from downloaded manga
        downloadedMangaList.forEach(item => {
            item.media?.genres?.forEach(genre => {
                if (genre) genreSet.add(genre)
            })
        })
        
        return Array.from(genreSet).sort()
    }, [downloadedMangaList])
    
    // Create collection list from downloaded manga only
    const combinedCollectionList = React.useMemo(() => {
        const lists: AL_MangaCollection_MediaListCollection_Lists[] = []
        
        // Add downloaded manga as a "Downloaded" list
        if (downloadedMangaList.length > 0) {
            const validDownloadedManga = downloadedMangaList.filter(item => item.media != null)
            if (validDownloadedManga.length > 0) {
                lists.push({
                    name: "Downloaded",
                    status: "DOWNLOADED" as AL_MediaListStatus,
                    entries: validDownloadedManga.map(item => ({
                        id: item.media!.id || 0,
                        mediaId: item.media!.id || 0,
                        media: item.media!,
                        listData: null,
                        libraryData: null,
                        nakamaLibraryData: null,
                    })),
                })
            }
        }
        
        return lists
    }, [downloadedMangaList])
    
    // Filter by genre if selected
    const filteredCollectionList = React.useMemo(() => {
        if (!params.genre?.length) return combinedCollectionList
        
        return combinedCollectionList.map(list => ({
            ...list,
            entries: list.entries?.filter(entry => 
                entry.media && entry.media.genres && entry.media.genres.some(genre => 
                    genre && params.genre?.includes(genre)
                )
            ) || [],
        })).filter(list => list.entries && list.entries.length > 0)
    }, [combinedCollectionList, params.genre])
    
    // No continue reading for downloaded manga only
    const continueReadingList: any[] = []
    
    const hasEntries = combinedCollectionList.some(list => list.entries && list.entries.length > 0)
    
    // No error handling needed for downloaded manga only
    
    if (isLoading) {
        return (
            <React.Fragment>
                <div className="p-4 space-y-4 relative z-[4]">
                    <Skeleton className="h-12 w-full max-w-lg relative" />
                    <div
                        className={cn(
                            "grid h-[22rem] min-[2000px]:h-[24rem] grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-7 min-[2000px]:grid-cols-8 gap-4",
                        )}
                    >
                        {[1, 2, 3, 4, 5, 6, 7, 8]?.map((_, idx) => {
                            return <Skeleton
                                key={idx} className={cn(
                                "h-[22rem] min-[2000px]:h-[24rem] col-span-1 aspect-[6/7] flex-none rounded-[--radius-md] relative overflow-hidden",
                                "[&:nth-child(8)]:hidden min-[2000px]:[&:nth-child(8)]:block",
                                "[&:nth-child(7)]:hidden 2xl:[&:nth-child(7)]:block",
                                "[&:nth-child(6)]:hidden xl:[&:nth-child(6)]:block",
                                "[&:nth-child(5)]:hidden xl:[&:nth-child(5)]:block",
                                "[&:nth-child(4)]:hidden lg:[&:nth-child(4)]:block",
                                "[&:nth-child(3)]:hidden md:[&:nth-child(3)]:block",
                            )}
                            />
                        })}
                    </div>
                </div>
            </React.Fragment>
        )
    }
    
    if (!hasEntries) {
        return (
            <div className="p-8 text-center">
                <span className="text-gray-400">No manga found in your library.</span>
            </div>
        )
    }
    
    return (
        <>
            {(
                !ts.disableLibraryScreenGenreSelector &&
                combinedCollectionList.flatMap(n => n.entries)?.length > 0 &&
                allGenres.length > 0
            ) && <MangaGenreSelector genres={allGenres} />}

            <PageWrapper key="manga-library-collection-lists" className="p-4 space-y-8 relative z-[4]" data-manga-library-collection-lists-container>
                <AnimatePresence mode="wait" initial={false}>
                    {!params.genre?.length ?
                        <MangaLibraryCollectionLists
                            key="manga-library-collection-lists"
                            collectionList={combinedCollectionList}
                            isLoading={isLoading}
                        />
                        : <MangaLibraryCollectionFilteredLists
                            key="manga-library-filtered-lists"
                            collectionList={filteredCollectionList}
                            isLoading={isLoading}
                        />
                    }
                </AnimatePresence>
            </PageWrapper>
        </>
    )
}

export default MangaLibraryView
