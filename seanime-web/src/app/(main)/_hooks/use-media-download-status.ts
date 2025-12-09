import { MediaDownloadStatus, useGetMediaDownloadingStatus } from "@/api/hooks/torrent_client.hooks"
import { useMemo } from "react"

/**
 * Hook to get the download status of a specific media item.
 * Uses the global media downloading status query.
 */
export function useMediaDownloadStatus(mediaId: number | undefined) {
    const { data: downloadingMedia } = useGetMediaDownloadingStatus(true)

    const status = useMemo(() => {
        if (!mediaId || !downloadingMedia) return null
        return downloadingMedia.find(item => item.mediaId === mediaId) || null
    }, [mediaId, downloadingMedia])

    return {
        isDownloading: status?.status === "downloading",
        isSeeding: status?.status === "seeding",
        isPaused: status?.status === "paused",
        isActive: !!status,
        status: status?.status || null,
        progress: status?.progress,
    }
}

/**
 * Hook to get all media download statuses as a map for efficient lookup.
 */
export function useAllMediaDownloadStatuses() {
    const { data: downloadingMedia, isLoading } = useGetMediaDownloadingStatus(true)

    const statusMap = useMemo(() => {
        const map = new Map<number, MediaDownloadStatus>()
        if (downloadingMedia) {
            for (const item of downloadingMedia) {
                map.set(item.mediaId, item)
            }
        }
        return map
    }, [downloadingMedia])

    return {
        statusMap,
        isLoading,
        getStatus: (mediaId: number) => statusMap.get(mediaId) || null,
        isActive: (mediaId: number) => statusMap.has(mediaId),
    }
}
