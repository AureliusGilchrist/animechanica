import { useServerMutation, useServerQuery } from "@/api/client/requests"
import { API_ENDPOINTS } from "@/api/generated/endpoints"
import { Manga_EnMasseDownloaderStatus } from "@/api/generated/types"

// Hook to get en masse downloader status
export function useGetEnMasseDownloaderStatus() {
    return useServerQuery<Manga_EnMasseDownloaderStatus>({
        endpoint: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.GetEnMasseDownloaderStatus.endpoint,
        method: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.GetEnMasseDownloaderStatus.methods[0],
        queryKey: [API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.GetEnMasseDownloaderStatus.key],
        enabled: true,
        refetchInterval: 2000, // Refetch every 2 seconds when running
    })
}

// Hook to start en masse downloader
export function useStartEnMasseDownloader() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StartEnMasseDownloader.endpoint,
        method: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StartEnMasseDownloader.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StartEnMasseDownloader.key],
    })
}

// Hook to stop en masse downloader
export function useStopEnMasseDownloader() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StopEnMasseDownloader.endpoint,
        method: API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StopEnMasseDownloader.methods[0],
        mutationKey: [API_ENDPOINTS.MANGA_EN_MASSE_DOWNLOADER.StopEnMasseDownloader.key],
    })
}
