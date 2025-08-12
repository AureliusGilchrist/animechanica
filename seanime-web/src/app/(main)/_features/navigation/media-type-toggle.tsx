"use client"
import React, { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { usePathname, useRouter } from "next/navigation"
import { cn } from "@/components/ui/core/styling"

type MediaType = "anime" | "manga"

export function MediaTypeToggle() {
    const pathname = usePathname()
    const router = useRouter()
    
    // Determine current media type based on pathname
    const getCurrentMediaType = (): MediaType => {
        if (pathname.startsWith("/manga")) {
            return "manga"
        }
        return "anime"
    }
    
    const [selectedMediaType, setSelectedMediaType] = useState<MediaType>(getCurrentMediaType())
    
    // Update selected type when pathname changes
    useEffect(() => {
        setSelectedMediaType(getCurrentMediaType())
    }, [pathname])
    
    const handleMediaTypeChange = (mediaType: MediaType) => {
        setSelectedMediaType(mediaType)
        
        if (mediaType === "anime") {
            router.push("/")
        } else {
            router.push("/manga")
        }
    }
    
    return (
        <div
            role="tablist"
            aria-label="Media type"
            className={cn(
                "relative inline-grid grid-cols-2 items-center rounded-full border",
                "bg-[--app-bg/0.6] backdrop-blur supports-[backdrop-filter]:bg-[--app-bg/0.4]",
                "border-[--border] p-1 h-9"
            )}
        >
            {/* Animated highlight */}
            <span
                aria-hidden
                className={cn(
                    "pointer-events-none absolute left-1 top-1 h-7 w-[calc(50%-0.25rem)] rounded-full",
                    "bg-[--primary-600]/15 ring-1 ring-inset ring-[--primary-400]/30",
                    "transition-transform duration-200 ease-out",
                    selectedMediaType === "manga" ? "translate-x-[calc(100%+0.5rem)]" : "translate-x-0"
                )}
            />
            <Button
                role="tab"
                aria-selected={selectedMediaType === "anime"}
                intent={selectedMediaType === "anime" ? "primary-subtle" : "gray-subtle"}
                size="sm"
                onClick={() => handleMediaTypeChange("anime")}
                className={cn(
                    "relative z-[1] px-4 py-1 text-sm rounded-full",
                    selectedMediaType === "anime" ? "text-[--primary-200]" : "text-[--muted] hover:text-foreground"
                )}
            >
                Anime
            </Button>
            <Button
                role="tab"
                aria-selected={selectedMediaType === "manga"}
                intent={selectedMediaType === "manga" ? "primary-subtle" : "gray-subtle"}
                size="sm"
                onClick={() => handleMediaTypeChange("manga")}
                className={cn(
                    "relative z-[1] px-4 py-1 text-sm rounded-full",
                    selectedMediaType === "manga" ? "text-[--primary-200]" : "text-[--muted] hover:text-foreground"
                )}
            >
                Manga
            </Button>
        </div>
    )
}
