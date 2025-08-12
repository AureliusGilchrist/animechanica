"use client"
import React, { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { usePathname, useRouter } from "next/navigation"

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
        <div className="flex items-center space-x-1 bg-muted p-1 rounded-lg">
            <Button
                intent={selectedMediaType === "anime" ? "primary" : "gray-subtle"}
                size="sm"
                onClick={() => handleMediaTypeChange("anime")}
                className="px-4 py-1 text-sm"
            >
                Anime
            </Button>
            <Button
                intent={selectedMediaType === "manga" ? "primary" : "gray-subtle"}
                size="sm"
                onClick={() => handleMediaTypeChange("manga")}
                className="px-4 py-1 text-sm"
            >
                Manga
            </Button>
        </div>
    )
}
