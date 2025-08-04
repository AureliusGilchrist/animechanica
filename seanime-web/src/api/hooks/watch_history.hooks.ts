import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

const API_BASE_URL = process.env.NODE_ENV === 'development' ? 'http://localhost:43211' : ''

// Types
export interface WatchHistoryEntry {
    mediaId: number
    mediaType: "anime" | "manga"
    episodeNumber?: number
    chapterNumber?: number
    progress: number // 0.0 to 1.0
    startDate: string
    lastWatched: string
    endDate?: string
    isCompleted: boolean
    totalDuration?: number // in seconds for anime
}

export interface ProgressUpdateRequest {
    mediaId: number
    mediaType: "anime" | "manga"
    episodeNumber?: number
    chapterNumber?: number
    progress: number
    duration?: number // total duration in seconds
}

export interface ProgressUpdateResponse {
    success: boolean
    progress: number
    autoCompleted: boolean
    message: string
}

export interface WatchHistoryResponse {
    history: WatchHistoryEntry[]
    total: number
}

// Update watch progress mutation
export function useUpdateWatchProgress() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (data: ProgressUpdateRequest): Promise<ProgressUpdateResponse> => {
            const response = await fetch(`${API_BASE_URL}/api/v1/watch-history/progress`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                credentials: "include",
                body: JSON.stringify(data),
            })

            if (!response.ok) {
                const error = await response.json() as { error?: string }
                throw new Error(error.error || "Failed to update progress")
            }

            return (await response.json()) as ProgressUpdateResponse
        },
        onSuccess: (data, variables) => {
            // Invalidate watch history queries
            queryClient.invalidateQueries({
                queryKey: ["watch-history"],
            })
            
            // Invalidate media watch status
            queryClient.invalidateQueries({
                queryKey: ["media-watch-status", variables.mediaId, variables.mediaType],
            })

            // Show auto-completion toast if applicable
            if (data.autoCompleted) {
                toast.success(`Auto-marked as ${variables.mediaType === "anime" ? "watched" : "read"} (80%+ progress)`)
            }
        },
        onError: (error: Error) => {
            toast.error(error.message || "Failed to update progress")
        },
    })
}

// Get watch history query
export function useGetWatchHistory(mediaType?: "anime" | "manga", limit?: number) {
    return useQuery({
        queryKey: ["watch-history", mediaType, limit],
        queryFn: async (): Promise<WatchHistoryResponse> => {
            const params = new URLSearchParams()
            if (mediaType) params.append("type", mediaType)
            if (limit) params.append("limit", limit.toString())

            const response = await fetch(
                `${API_BASE_URL}/api/v1/watch-history?${params.toString()}`,
                {
                    method: "GET",
                    credentials: "include",
                }
            )

            if (!response.ok) {
                throw new Error("Failed to get watch history")
            }

            return (await response.json()) as WatchHistoryResponse
        },
    })
}

// Get media watch status query
export function useGetMediaWatchStatus(mediaId: number, mediaType: "anime" | "manga") {
    return useQuery({
        queryKey: ["media-watch-status", mediaId, mediaType],
        queryFn: async (): Promise<WatchHistoryEntry> => {
            const response = await fetch(
                `${API_BASE_URL}/api/v1/watch-history/media/${mediaId}?type=${mediaType}`,
                {
                    method: "GET",
                    credentials: "include",
                }
            )

            if (!response.ok) {
                throw new Error("Failed to get media watch status")
            }

            return (await response.json()) as WatchHistoryEntry
        },
        enabled: mediaId > 0 && (mediaType === "anime" || mediaType === "manga"),
    })
}
