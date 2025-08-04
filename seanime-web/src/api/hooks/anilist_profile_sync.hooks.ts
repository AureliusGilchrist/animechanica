import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

const API_BASE_URL = process.env.NODE_ENV === 'development' ? 'http://localhost:43211' : ''

// Types
export interface UpdateAniListProfileRequest {
    about?: string
    avatar?: string
    bannerImage?: string
}

export interface UpdateAniListProfileResponse {
    success: boolean
    message: string
}

export interface ToggleFavoriteAniListRequest {
    mediaId: number
    mediaType: "anime" | "manga" | "character" | "staff" | "studio"
}

export interface ToggleFavoriteAniListResponse {
    success: boolean
    message: string
    isFavorite: boolean
}

// Update AniList profile mutation
export function useUpdateAniListProfile() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (data: UpdateAniListProfileRequest): Promise<UpdateAniListProfileResponse> => {
            const response = await fetch(`${API_BASE_URL}/api/v1/anilist/profile/update`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                credentials: "include",
                body: JSON.stringify(data),
            })

            if (!response.ok) {
                const error = await response.json() as { error?: string }
                throw new Error(error.error || "Failed to update AniList profile")
            }

            return (await response.json()) as UpdateAniListProfileResponse
        },
        onSuccess: (data) => {
            // Invalidate user profile queries to refresh data
            queryClient.invalidateQueries({
                queryKey: ["anilist-user"],
            })
            
            queryClient.invalidateQueries({
                queryKey: ["anilist-user-favorites"],
            })

            // Show success toast
            toast.success(data.message)
        },
        onError: (error: Error) => {
            toast.error(error.message || "Failed to update AniList profile")
        },
    })
}

// Toggle favorite on AniList mutation
export function useToggleFavoriteAniList() {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (data: ToggleFavoriteAniListRequest): Promise<ToggleFavoriteAniListResponse> => {
            const response = await fetch(`${API_BASE_URL}/api/v1/anilist/favorites/toggle`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                credentials: "include",
                body: JSON.stringify(data),
            })

            if (!response.ok) {
                const error = await response.json() as { error?: string }
                throw new Error(error.error || "Failed to toggle favorite on AniList")
            }

            return (await response.json()) as ToggleFavoriteAniListResponse
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
            toast.error(error.message || "Failed to toggle favorite on AniList")
        },
    })
}
