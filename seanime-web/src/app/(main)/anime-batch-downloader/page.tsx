"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"

export default function AnimeBatchDownloaderPage() {
    const router = useRouter()
    
    useEffect(() => {
        // Redirect to en-masse-downloader with anime tab selected
        router.replace("/en-masse-downloader?tab=anime")
    }, [router])
    
    // Show loading while redirecting
    return (
        <div className="container mx-auto p-6 space-y-6">
            <div className="flex items-center justify-center min-h-[400px]">
                <div className="text-center">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
                    <p className="text-muted-foreground">Redirecting to En Masse Downloader...</p>
                </div>
            </div>
        </div>
    )
}
