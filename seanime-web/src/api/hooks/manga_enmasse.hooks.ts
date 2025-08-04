import { API_ENDPOINTS } from "@/api/generated/endpoints"
import { useServerMutation, useServerQuery } from "@/api/client/requests"

export interface EnMasseStatus {
    isRunning: boolean
    isPaused: boolean
    status: string
    currentSeries: string
    processedCount: number
    totalCount: number
    errorCount: number
    processedSeries: string[]
    errorSeries: string[]
}

// Get En Masse Downloader status
export function useGetEnMasseStatus() {
    return useServerQuery<EnMasseStatus>({
        queryKey: ["en-masse-status"],
        endpoint: API_ENDPOINTS.EN_MASSE_DOWNLOADER.GetEnMasseStatus.endpoint,
        method: API_ENDPOINTS.EN_MASSE_DOWNLOADER.GetEnMasseStatus.methods[0] as "GET",
        refetchInterval: 2000, // Refetch every 2 seconds when running
    })
}

// Start En Masse Download
export function useStartEnMasseDownload() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.EN_MASSE_DOWNLOADER.StartEnMasseDownload.endpoint,
        method: API_ENDPOINTS.EN_MASSE_DOWNLOADER.StartEnMasseDownload.methods[0] as "POST",
        mutationKey: ["start-en-masse-download"],
    })
}

// Pause En Masse Download
export function usePauseEnMasseDownload() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.EN_MASSE_DOWNLOADER.PauseEnMasseDownload.endpoint,
        method: API_ENDPOINTS.EN_MASSE_DOWNLOADER.PauseEnMasseDownload.methods[0] as "POST",
        mutationKey: ["pause-en-masse-download"],
    })
}

// Resume En Masse Download
export function useResumeEnMasseDownload() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.EN_MASSE_DOWNLOADER.ResumeEnMasseDownload.endpoint,
        method: API_ENDPOINTS.EN_MASSE_DOWNLOADER.ResumeEnMasseDownload.methods[0] as "POST",
        mutationKey: ["resume-en-masse-download"],
    })
}

// Stop En Masse Download
export function useStopEnMasseDownload() {
    return useServerMutation<boolean>({
        endpoint: API_ENDPOINTS.EN_MASSE_DOWNLOADER.StopEnMasseDownload.endpoint,
        method: API_ENDPOINTS.EN_MASSE_DOWNLOADER.StopEnMasseDownload.methods[0] as "POST",
        mutationKey: ["stop-en-masse-download"],
    })
}
