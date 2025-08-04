import React from "react"
import { Button } from "@/components/ui/button"
import { IconButton } from "@/components/ui/button"
import { useGetFavoriteStatus, useToggleFavorite } from "@/api/hooks/favorites.hooks"
import { BiHeart, BiSolidHeart } from "react-icons/bi"
import { Spinner } from "@/components/ui/loading-spinner"
import { cn } from "@/components/ui/core/styling"

interface FavoriteButtonProps {
    mediaId: number
    mediaType: "anime" | "manga"
    variant?: "button" | "icon"
    size?: "sm" | "md" | "lg"
    className?: string
    showText?: boolean
}

export function FavoriteButton({
    mediaId,
    mediaType,
    variant = "button",
    size = "md",
    className,
    showText = true,
}: FavoriteButtonProps) {
    const { data: favoriteStatus, isLoading: isLoadingStatus } = useGetFavoriteStatus(mediaId, mediaType)
    const { mutate: toggleFavorite, isPending: isToggling } = useToggleFavorite()

    const isFavorite = favoriteStatus?.isFavorite || false
    const isLoading = isLoadingStatus || isToggling

    const handleToggle = () => {
        if (isLoading) return

        toggleFavorite({
            mediaId,
            mediaType,
            action: isFavorite ? "remove" : "add",
        })
    }

    const HeartIcon = isFavorite ? BiSolidHeart : BiHeart
    const text = isFavorite ? "Remove from Favorites" : "Add to Favorites"

    if (variant === "icon") {
        return (
            <IconButton
                onClick={handleToggle}
                disabled={isLoading}
                size={size}
                className={cn(
                    "transition-colors duration-200",
                    isFavorite 
                        ? "text-red-500 hover:text-red-600" 
                        : "text-gray-400 hover:text-red-500",
                    className
                )}
                icon={isLoading ? (
                    <Spinner className="w-4 h-4" />
                ) : (
                    <HeartIcon className="w-5 h-5" />
                )}
                aria-label={text}
            />
        )
    }

    return (
        <Button
            onClick={handleToggle}
            disabled={isLoading}
            intent={isFavorite ? "gray-outline" : "gray-outline"}
            size={size}
            className={cn(
                "transition-all duration-200",
                isFavorite 
                    ? "border-red-500 text-red-500 hover:bg-red-500 hover:text-white" 
                    : "border-gray-600 text-gray-300 hover:border-red-500 hover:text-red-500",
                className
            )}
        >
            {isLoading ? (
                <Spinner className="w-4 h-4 mr-2" />
            ) : (
                <HeartIcon className={cn("w-4 h-4", showText && "mr-2")} />
            )}
            {showText && (isLoading ? "Updating..." : text)}
        </Button>
    )
}
