import { useQuery, useMutation, useQueryClient, UseQueryResult } from "@tanstack/react-query"
import { useEffect } from "react"
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
export function useGetDownloadedMangaSeries(): UseQueryResult<DownloadedMangaSeries[]> {
    const serverStatus = useServerStatus()
    // LocalStorage cache helpers
    const LS_KEY = "downloadedMangaSeries"
    const TTL_MS = 1000 * 60 * 60 * 12 // 12 hours

    type Stored = { data: DownloadedMangaSeries[]; ts: number }

    function readCache(): { data: DownloadedMangaSeries[] | undefined; ts?: number } {
        if (typeof window === "undefined") return { data: undefined }
        try {
            const raw = window.localStorage.getItem(LS_KEY)
            if (!raw) return { data: undefined }
            const parsed = JSON.parse(raw) as Stored
            if (!parsed || !Array.isArray(parsed.data) || typeof parsed.ts !== "number") return { data: undefined }
            const age = Date.now() - parsed.ts
            if (age > TTL_MS) {
                window.localStorage.removeItem(LS_KEY)
                return { data: undefined }
            }
            return { data: parsed.data, ts: parsed.ts }
        } catch {
            return { data: undefined }
        }
    }

    function writeCache(data: DownloadedMangaSeries[]) {
        if (typeof window === "undefined") return
        try {
            const payload: Stored = { data, ts: Date.now() }
            window.localStorage.setItem(LS_KEY, JSON.stringify(payload))
        } catch {
            // ignore quota/JSON errors
        }
    }

    const cached = readCache()

    const query = useQuery<DownloadedMangaSeries[]>({
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
        staleTime: 5 * 60 * 1000, // 5 minutes
        // Hydrate instantly from cache if present without affecting types
        placeholderData: cached.data as DownloadedMangaSeries[] | undefined,
    })

    // Persist to localStorage whenever fresh data arrives
    useEffect(() => {
        const d = query.data as DownloadedMangaSeries[] | undefined
        if (Array.isArray(d)) {
            writeCache(d)
        }
    }, [query.data])

    return query
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
