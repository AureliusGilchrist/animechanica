import { useServerMutation, useServerQuery } from "@/api/client/requests"
import { toast } from "sonner"

export type NyaaCrawlerStatus = {
    isRunning: boolean
    progress: number
    currentQuery: string
    totalQueries: number
    processedQueries: number
    torrentsFound: number
    torrentsAdded: number
    startTime: string
    estimatedTimeLeft: string
    lastActivity: string
    logs: string[]
}

export type NyaaCrawlerConfig = {
    searchQueries: string[]
    startPage: number
    endPage: number
    delaySeconds: number
    qbittorrentUrl: string
    qbittorrentUser: string
    qbittorrentPass: string
    downloadPath: string
}

// Get crawler status
export function useGetNyaaCrawlerStatus() {
    return useServerQuery<NyaaCrawlerStatus>({
        endpoint: "/api/v1/nyaa-crawler/status",
        method: "GET",
        queryKey: ["nyaa-crawler-status"],
        refetchInterval: 2000, // Poll every 2 seconds
    })
}

// Get crawler config
export function useGetNyaaCrawlerConfig() {
    return useServerQuery<NyaaCrawlerConfig>({
        endpoint: "/api/v1/nyaa-crawler/config",
        method: "GET",
        queryKey: ["nyaa-crawler-config"],
    })
}

// Update crawler config
export function useUpdateNyaaCrawlerConfig() {
    return useServerMutation<{ success: boolean; message: string }>({
        endpoint: "/api/v1/nyaa-crawler/config",
        method: "POST",
        mutationKey: ["update-nyaa-crawler-config"],
        onSuccess: () => {
            toast.success("Configuration updated successfully")
        },
        onError: (error) => {
            toast.error(`Failed to update configuration: ${error.message}`)
        },
    })
}

// Start crawler
export function useStartNyaaCrawler() {
    return useServerMutation<{ success: boolean; message: string; status: NyaaCrawlerStatus }>({
        endpoint: "/api/v1/nyaa-crawler/start",
        method: "POST",
        mutationKey: ["start-nyaa-crawler"],
        onSuccess: () => {
            toast.success("Nyaa crawler started successfully")
        },
        onError: (error) => {
            toast.error(`Failed to start crawler: ${error.message}`)
        },
    })
}

// Stop crawler
export function useStopNyaaCrawler() {
    return useServerMutation<{ success: boolean; message: string; status: NyaaCrawlerStatus }>({
        endpoint: "/api/v1/nyaa-crawler/stop",
        method: "POST",
        mutationKey: ["stop-nyaa-crawler"],
        onSuccess: () => {
            toast.success("Nyaa crawler stopped successfully")
        },
        onError: (error) => {
            toast.error(`Failed to stop crawler: ${error.message}`)
        },
    })
}
