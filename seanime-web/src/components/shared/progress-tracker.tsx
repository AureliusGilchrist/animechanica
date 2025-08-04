import React, { useEffect, useRef, useCallback } from "react"
import { useUpdateWatchProgress } from "@/api/hooks/watch_history.hooks"
import { useDebounce } from "@/hooks/use-debounce"

interface ProgressTrackerProps {
    mediaId: number
    mediaType: "anime" | "manga"
    episodeNumber?: number
    chapterNumber?: number
    duration?: number // total duration in seconds for video
    onProgressUpdate?: (progress: number) => void
    onAutoComplete?: () => void
    trackingInterval?: number // in milliseconds, default 5000 (5 seconds)
}

export function ProgressTracker({
    mediaId,
    mediaType,
    episodeNumber,
    chapterNumber,
    duration,
    onProgressUpdate,
    onAutoComplete,
    trackingInterval = 5000,
}: ProgressTrackerProps) {
    const { mutate: updateProgress } = useUpdateWatchProgress()
    const progressRef = useRef<number>(0)
    const lastUpdateRef = useRef<number>(0)
    const intervalRef = useRef<NodeJS.Timeout>()
    const autoCompletedRef = useRef<boolean>(false)

    // Debounce progress updates to avoid too many API calls
    const debouncedProgress = useDebounce(progressRef.current, 2000)

    const sendProgressUpdate = useCallback((progress: number) => {
        if (Math.abs(progress - lastUpdateRef.current) < 0.05) {
            // Don't update if progress change is less than 5%
            return
        }

        updateProgress(
            {
                mediaId,
                mediaType,
                episodeNumber,
                chapterNumber,
                progress,
                duration,
            },
            {
                onSuccess: (data) => {
                    lastUpdateRef.current = progress
                    
                    if (data.autoCompleted && !autoCompletedRef.current) {
                        autoCompletedRef.current = true
                        onAutoComplete?.()
                    }
                },
            }
        )
    }, [mediaId, mediaType, episodeNumber, chapterNumber, duration, updateProgress, onAutoComplete])

    // Update progress when debounced value changes
    useEffect(() => {
        if (debouncedProgress > 0) {
            sendProgressUpdate(debouncedProgress)
        }
    }, [debouncedProgress, sendProgressUpdate])

    // Method to update progress (called by parent components)
    const updateProgressValue = useCallback((progress: number) => {
        progressRef.current = Math.max(0, Math.min(1, progress))
        onProgressUpdate?.(progressRef.current)
    }, [onProgressUpdate])

    // Auto-tracking for video elements (anime)
    useEffect(() => {
        if (mediaType !== "anime" || !duration) return

        const trackVideoProgress = () => {
            const videoElements = document.querySelectorAll("video")
            
            videoElements.forEach((video) => {
                if (video.duration && video.currentTime) {
                    const progress = video.currentTime / video.duration
                    updateProgressValue(progress)
                }
            })
        }

        // Set up interval for video progress tracking
        intervalRef.current = setInterval(trackVideoProgress, trackingInterval)

        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current)
            }
        }
    }, [mediaType, duration, trackingInterval, updateProgressValue])

    // Scroll-based tracking for manga
    useEffect(() => {
        if (mediaType !== "manga") return

        const trackScrollProgress = () => {
            const scrollTop = window.pageYOffset || document.documentElement.scrollTop
            const scrollHeight = document.documentElement.scrollHeight - document.documentElement.clientHeight
            
            if (scrollHeight > 0) {
                const progress = scrollTop / scrollHeight
                updateProgressValue(progress)
            }
        }

        // Add scroll listener for manga progress tracking
        window.addEventListener("scroll", trackScrollProgress, { passive: true })

        return () => {
            window.removeEventListener("scroll", trackScrollProgress)
        }
    }, [mediaType, updateProgressValue])

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current)
            }
            
            // Send final progress update if there's unsaved progress
            if (progressRef.current > lastUpdateRef.current) {
                sendProgressUpdate(progressRef.current)
            }
        }
    }, [sendProgressUpdate])

    // Note: This component doesn't expose imperative methods
    // Use the useProgressTracker hook instead for manual progress updates

    // This component doesn't render anything visible
    return null
}

// Hook for manual progress tracking
export function useProgressTracker(
    mediaId: number,
    mediaType: "anime" | "manga",
    episodeNumber?: number,
    chapterNumber?: number,
    duration?: number
) {
    const { mutate: updateProgress } = useUpdateWatchProgress()

    const trackProgress = useCallback((progress: number) => {
        updateProgress({
            mediaId,
            mediaType,
            episodeNumber,
            chapterNumber,
            progress: Math.max(0, Math.min(1, progress)),
            duration,
        })
    }, [mediaId, mediaType, episodeNumber, chapterNumber, duration, updateProgress])

    return { trackProgress }
}
