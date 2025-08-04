import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

const API_BASE_URL = process.env.NODE_ENV === 'development' ? 'http://localhost:43211' : ''

// Types
export interface FavoriteRequest {
    mediaId: number
    mediaType: "anime" | "manga"
    action: "add" | "remove"
}

export interface FavoriteResponse {
    success: boolean
    message: string
    isFavorite: boolean
}

export interface FavoriteStatus {
    isFavorite: boolean
    mediaId: number
    mediaType: string
}

// Toggle favorite mutation
export function useToggleFavorite() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (data: FavoriteRequest): Promise<FavoriteResponse> => {
            const response = await fetch(`${API_BASE_URL}/api/v1/favorites/toggle`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                credentials: "include",
                body: JSON.stringify(data),
            })

            if (!response.ok) {
                const error = await response.json() as { error?: string }
                throw new Error(error.error || "Failed to toggle favorite")
            }

            return (await response.json()) as FavoriteResponse
        },
        onSuccess: (data, variables) => {
            // Invalidate favorite status queries
            queryClient.invalidateQueries({
                queryKey: ["favorite-status", variables.mediaId, variables.mediaType],
            })
            
            // Invalidate profile favorites
            queryClient.invalidateQueries({
                queryKey: ["anilist-user-favorites"],
            })

            // Show success toast
            toast.success(data.message)
        },
        onError: (error: Error) => {
            toast.error(error.message || "Failed to update favorite")
        },
    })
}

// Get favorite status query
export function useGetFavoriteStatus(mediaId: number, mediaType: "anime" | "manga") {
    return useQuery({
        queryKey: ["favorite-status", mediaId, mediaType],
        queryFn: async (): Promise<FavoriteStatus> => {
            const response = await fetch(
                `${API_BASE_URL}/api/v1/favorites/status/${mediaId}?type=${mediaType}`,
                {
                    method: "GET",
                    credentials: "include",
                }
            )

            if (!response.ok) {
                throw new Error("Failed to get favorite status")
            }

            return (await response.json()) as FavoriteStatus
        },
        enabled: mediaId > 0 && (mediaType === "anime" || mediaType === "manga"),
    })
}
