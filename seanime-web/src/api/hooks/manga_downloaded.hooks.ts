import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useServerStatus } from "@/app/(main)/_hooks/use-server-status"

export type DownloadedMangaSeries = {
    seriesTitle: string
    seriesPath: string
    mediaId?: number
    coverImagePath?: string
    chapterCount: number
    chapters: DownloadedMangaChapter[]
    lastUpdated: number
}

export type DownloadedMangaChapter = {
    chapterNumber: string
    chapterTitle: string
    chapterPath: string
    pageCount: number
    lastModified: number
}

// Hook to get downloaded manga series
export function useGetDownloadedMangaSeries() {
    const serverStatus = useServerStatus()
    
    return useQuery<DownloadedMangaSeries[]>({
        queryKey: ["downloaded-manga-series"],
        queryFn: async (): Promise<DownloadedMangaSeries[]> => {
            const res = await fetch("/api/v1/manga/downloaded-series")
            if (!res.ok) {
                throw new Error("Failed to fetch downloaded manga series")
            }
            const data = await res.json()
            return data as DownloadedMangaSeries[]
        },
        enabled: serverStatus?.isOffline === false,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        refetchOnMount: false,
        staleTime: 30 * 60 * 1000, // 30 minutes
        gcTime: 2 * 60 * 60 * 1000, // 2 hours (cache retention)
    })
}

// Hook to refresh downloaded manga cache
export function useRefreshDownloadedMangaCache() {
    const queryClient = useQueryClient()
    
    return useMutation<boolean, Error>({
        mutationFn: async (): Promise<boolean> => {
            const res = await fetch("/api/v1/manga/refresh-downloaded-cache", {
                method: "POST",
            })
            if (!res.ok) {
                throw new Error("Failed to refresh downloaded manga cache")
            }
            const data = await res.json()
            return data as boolean
        },
        onSuccess: () => {
            // Invalidate and refetch downloaded manga series
            queryClient.invalidateQueries({ queryKey: ["downloaded-manga-series"] })
        },
    })
}
