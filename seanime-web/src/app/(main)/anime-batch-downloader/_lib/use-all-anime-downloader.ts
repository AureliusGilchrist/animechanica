"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
// import { useWebsocketSender } from "@/app/(main)/_hooks/handle-websockets"

export interface AllAnimeDownloadSettings {
    preferDualAudio: boolean
    preferBluray: boolean
    preferHighestRes: boolean
    minSeeders: number
    maxConcurrentBatches: number
    skipOva: boolean
    skipSpecials: boolean
    minYear: number
    maxYear: number
    includeGenres: string[]
    excludeGenres: string[]
}

export interface DownloadLogEntry {
    animeTitle: string
    query: string
    status: "failed" | "success" | "info"
    message: string
    time: string
}

export interface AllAnimeDownloadJob {
    id: string
    status: "pending" | "running" | "completed" | "failed" | "cancelled" | "paused"
    totalAnime: number
    completedAnime: number
    failedAnime: number
    activeBatches: number
    progress: number
    startTime: string
    endTime?: string
    settings: AllAnimeDownloadSettings
    statistics?: AllAnimeDownloadStats
    logs?: DownloadLogEntry[]
    error?: string
}

export interface AllAnimeDownloadStats {
    totalSizeGb: number
    downloadedSizeGb: number
    averageSpeed: number
    estimatedTimeLeft: string
    dualAudioCount: number
    blurayCount: number
    highResCount: number
    torrentsAdded: number
    qbittorrentActive: number
}

export interface DatabaseStatus {
    loaded: boolean
    totalAnime?: number
    databasePath?: string
    sizeFormatted?: string
    lastModified?: string
    path?: string
    stats?: {
        totalAnime: number
        byType: Record<string, number>
        byStatus: Record<string, number>
        yearRange: { min: number; max: number }
    }
}

export interface DownloadPreview {
    totalInDatabase: number
    willDownload: number
    filtered: number
    estimatedSizeGB: number
    estimatedDuration: string
    typeBreakdown: Record<string, number>
    yearBreakdown: Record<string, number>
    settings: AllAnimeDownloadSettings
    preferences: {
        dualAudio: boolean
        bluray: boolean
        highestRes: boolean
    }
}

const defaultSettings: AllAnimeDownloadSettings = {
    preferDualAudio: true,
    preferBluray: true,
    preferHighestRes: true,
    minSeeders: 5,
    maxConcurrentBatches: 3,
    skipOva: false,
    skipSpecials: false,
    minYear: 1990,
    maxYear: 2024,
    includeGenres: [],
    excludeGenres: [],
}

// Settings presets for common configurations
export const settingsPresets = {
    conservative: {
        ...defaultSettings,
        minSeeders: 10,
        maxConcurrentBatches: 2,
        preferDualAudio: true,
        preferBluray: true,
        preferHighestRes: true,
        skipOva: true,
        skipSpecials: true,
        minYear: 2000,
    },
    balanced: {
        ...defaultSettings,
        minSeeders: 5,
        maxConcurrentBatches: 3,
        preferDualAudio: true,
        preferBluray: false,
        preferHighestRes: false,
        skipOva: false,
        skipSpecials: false,
    },
    aggressive: {
        ...defaultSettings,
        minSeeders: 1,
        maxConcurrentBatches: 5,
        preferDualAudio: false,
        preferBluray: false,
        preferHighestRes: false,
        skipOva: false,
        skipSpecials: false,
        minYear: 1960,
    },
    qualityFocused: {
        ...defaultSettings,
        minSeeders: 15,
        maxConcurrentBatches: 2,
        preferDualAudio: true,
        preferBluray: true,
        preferHighestRes: true,
        skipOva: true,
        skipSpecials: true,
        minYear: 2010,
    },
}

// Available genres for filtering
export const availableGenres = [
    'Action', 'Adventure', 'Comedy', 'Drama', 'Fantasy', 'Horror', 'Mystery', 'Romance', 'Sci-Fi', 'Slice of Life',
    'Sports', 'Supernatural', 'Thriller', 'Ecchi', 'Harem', 'Hentai', 'Josei', 'Kids', 'Magic', 'Martial Arts',
    'Mecha', 'Military', 'Music', 'Parody', 'Police', 'Psychological', 'School', 'Seinen', 'Shoujo', 'Shounen',
    'Space', 'Super Power', 'Vampire', 'Yaoi', 'Yuri', 'Dementia', 'Demons', 'Game', 'Historical', 'Samurai'
]

// Settings persistence key
const SETTINGS_STORAGE_KEY = 'anime-batch-downloader-settings'

// Helper functions for settings persistence
function saveSettingsToStorage(settings: AllAnimeDownloadSettings) {
    try {
        localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings))
    } catch (error) {
        console.warn('Failed to save settings to localStorage:', error)
    }
}

function loadSettingsFromStorage(): AllAnimeDownloadSettings {
    try {
        const stored = localStorage.getItem(SETTINGS_STORAGE_KEY)
        if (stored) {
            const parsed = JSON.parse(stored)
            // Ensure parsed is an object before spreading
            if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                // Merge with defaults to ensure all properties exist
                return { ...defaultSettings, ...parsed }
            }
        }
    } catch (error) {
        console.warn('Failed to load settings from localStorage:', error)
    }
    return defaultSettings
}

export function useAllAnimeDownloader() {
    const [databaseStatus, setDatabaseStatus] = useState<DatabaseStatus>({ loaded: false })
    const [activeJob, setActiveJob] = useState<AllAnimeDownloadJob | null>(null)
    const [settings, setSettings] = useState<AllAnimeDownloadSettings>(() => loadSettingsFromStorage())
    const [preview, setPreview] = useState<DownloadPreview | null>(null)
    const [isLoading, setIsLoading] = useState(false)
    const [databaseStats, setDatabaseStats] = useState<any>(null)

    // Re-entrancy guards to prevent rapid duplicate requests
    const isStartingRef = useRef(false)
    const isCancellingRef = useRef(false)

    // const { sendMessage } = useWebsocketSender() // Temporarily disabled for build

    // Load database
    const loadDatabase = useCallback(async (databasePath?: string) => {
        setIsLoading(true)
        try {
            const response = await fetch("/api/v1/anime/database/load", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    databasePath: databasePath || "/aeternae/library/manga/seanime/anime-offline-database-minified.json"
                }),
            })

            if (!response.ok) {
                throw new Error("Failed to load database")
            }

            const result = await response.json()
            
            // Get database stats
            const statsResponse = await fetch("/api/v1/anime/database/stats")
            const stats = statsResponse.ok ? await statsResponse.json() : null

            setDatabaseStatus({
                loaded: true,
                totalAnime: (result as any).totalAnime,
                databasePath: (result as any).databasePath,
                stats: stats as any,
            })

            toast.success(`Database loaded successfully! ${(result as any).totalAnime} anime found.`)
        } catch (error) {
            console.error("Failed to load database:", error)
            toast.error("Failed to load anime database")
        } finally {
            setIsLoading(false)
        }
    }, [])

    // Start download
    const startDownload = useCallback(async (customSettings?: Partial<AllAnimeDownloadSettings>) => {
        // Prevent duplicate starts if already starting or a job is active
        if (isStartingRef.current) return
        if (activeJob && (activeJob.status === "running" || activeJob.status === "pending" || activeJob.status === "paused")) {
            toast.info("A batch download is already in progress")
            return
        }
        isStartingRef.current = true
        setIsLoading(true)
        try {
            const downloadSettings = { ...settings, ...customSettings }

            // Build a plain, serializable settings object to avoid cyclic references/proxies
            const plainSettings: AllAnimeDownloadSettings = {
                preferDualAudio: Boolean((downloadSettings as any)?.preferDualAudio),
                preferBluray: Boolean((downloadSettings as any)?.preferBluray),
                preferHighestRes: Boolean((downloadSettings as any)?.preferHighestRes),
                minSeeders: Number((downloadSettings as any)?.minSeeders ?? defaultSettings.minSeeders),
                maxConcurrentBatches: Number((downloadSettings as any)?.maxConcurrentBatches ?? defaultSettings.maxConcurrentBatches),
                skipOva: Boolean((downloadSettings as any)?.skipOva),
                skipSpecials: Boolean((downloadSettings as any)?.skipSpecials),
                minYear: Number((downloadSettings as any)?.minYear ?? defaultSettings.minYear),
                maxYear: Number((downloadSettings as any)?.maxYear ?? defaultSettings.maxYear),
                includeGenres: Array.isArray((downloadSettings as any)?.includeGenres) ? [ ...(downloadSettings as any).includeGenres ] : [],
                excludeGenres: Array.isArray((downloadSettings as any)?.excludeGenres) ? [ ...(downloadSettings as any).excludeGenres ] : [],
            }

            // Defensive pre-serialization check to catch cyclic structures early
            let serializedBody = ""
            try {
                serializedBody = JSON.stringify(plainSettings)
            } catch (serr) {
                console.error("Settings are not serializable:", { settings, customSettings, error: serr })
                toast.error("Settings are not serializable. Please adjust and try again.")
                return
            }

            // Debug log for endpoint and outgoing body
            try {
                console.log("Starting all-anime download", { url: "/api/v1/anime/download-all", body: plainSettings })
            } catch {}

            const response = await fetch("/api/v1/anime/download-all", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: serializedBody,
            })

            if (!response.ok) {
                // Attempt to parse error response for better messaging
                try {
                    const errBody = await response.json() as any
                    const errData = errBody?.data || errBody
                    const msg: string = errData?.error || errData?.message || "Failed to start all-anime download"
                    if (typeof msg === "string" && msg.toLowerCase().includes("already in progress")) {
                        toast.info(msg)
                        if (errData?.job) {
                            setActiveJob(errData.job as AllAnimeDownloadJob)
                        }
                        return
                    }
                    toast.error(msg)
                    return
                } catch (_) {
                    // Fallback: try text
                    try {
                        const txt = await response.text()
                        if (txt.toLowerCase().includes("already in progress")) {
                            toast.info(txt)
                            return
                        }
                        toast.error(txt || "Failed to start all-anime download")
                        return
                    } catch {
                        // Give up, throw generic
                        throw new Error("Failed to start download")
                    }
                }
            }

            const result = await response.json() as any
            const jobData = result.data || result

            // If backend indicates failure in body, surface the error
            if (jobData && jobData.success === false) {
                const msg = jobData.error || "Failed to start all-anime download"
                toast.error(msg)
                return
            }

            // If backend returns existing job with info message
            if (jobData && typeof jobData.message === "string" && jobData.message.toLowerCase().includes("already in progress")) {
                toast.info(jobData.message)
            } else {
                toast.success("All-anime download started! This will take a very long time.")
            }

            if (jobData?.job) {
                setActiveJob(jobData.job as AllAnimeDownloadJob)
            }
        } catch (error) {
            console.error("Failed to start download:", error)
            toast.error("Failed to start all-anime download")
        } finally {
            setIsLoading(false)
            isStartingRef.current = false
        }
    }, [settings, activeJob])

    // Cancel download
    const cancelDownload = useCallback(async () => {
        if (!activeJob) return
        if (isCancellingRef.current) return
        isCancellingRef.current = true
        setIsLoading(true)
        try {
            const response = await fetch("/api/v1/anime/download-all/cancel", {
                method: "POST",
            })

            if (!response.ok) {
                throw new Error("Failed to cancel download")
            }
            const result = await response.json() as any
            const data = result?.data || result
            const msg: string = data?.message || "All-anime download cancelled"
            const job = data?.job as AllAnimeDownloadJob | undefined

            // Update local state to reflect cancellation immediately
            if (job) {
                setActiveJob({ ...job, status: "cancelled" })
            } else {
                // If backend returned no job, clear local job if any
                setActiveJob(prev => (prev ? { ...prev, status: "cancelled" } : prev))
            }

            toast.success(msg)
        } catch (error) {
            console.error("Failed to cancel download:", error)
            toast.error("Failed to cancel download")
        } finally {
            setIsLoading(false)
            isCancellingRef.current = false
        }
    }, [activeJob])

    // Update settings with persistence
    const updateSettings = useCallback((newSettings: Partial<AllAnimeDownloadSettings>) => {
        setSettings(prev => {
            const updated = { ...prev, ...newSettings }
            saveSettingsToStorage(updated)
            return updated
        })
        setPreview(null) // Clear preview when settings change
    }, [])

    // Apply settings preset
    const applyPreset = useCallback((presetName: keyof typeof settingsPresets) => {
        const preset = settingsPresets[presetName]
        setSettings(preset)
        saveSettingsToStorage(preset)
        setPreview(null)
        toast.success(`Applied ${presetName} preset`)
    }, [])

    // Reset settings to defaults
    const resetSettings = useCallback(() => {
        setSettings(defaultSettings)
        saveSettingsToStorage(defaultSettings)
        setPreview(null)
        toast.success('Settings reset to defaults')
    }, [])

    // Export settings to JSON
    const exportSettings = useCallback(() => {
        try {
            const dataStr = JSON.stringify(settings, null, 2)
            const dataBlob = new Blob([dataStr], { type: 'application/json' })
            const url = URL.createObjectURL(dataBlob)
            const link = document.createElement('a')
            link.href = url
            link.download = 'anime-batch-downloader-settings.json'
            document.body.appendChild(link)
            link.click()
            document.body.removeChild(link)
            URL.revokeObjectURL(url)
            toast.success('Settings exported successfully')
        } catch (error) {
            console.error('Failed to export settings:', error)
            toast.error('Failed to export settings')
        }
    }, [settings])

    // Import settings from JSON
    const importSettings = useCallback((file: File) => {
        const reader = new FileReader()
        reader.onload = (e) => {
            try {
                const content = e.target?.result as string
                const imported = JSON.parse(content) as AllAnimeDownloadSettings
                
                // Validate imported settings have required properties
                const merged = { ...defaultSettings, ...imported }
                setSettings(merged)
                saveSettingsToStorage(merged)
                setPreview(null)
                toast.success('Settings imported successfully')
            } catch (error) {
                console.error('Failed to import settings:', error)
                toast.error('Failed to import settings - invalid file format')
            }
        }
        reader.readAsText(file)
    }, [])

    // Generate preview
    const generatePreview = useCallback(async () => {
        setIsLoading(true)
        try {
            const response = await fetch("/api/v1/anime/download-all/preview", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ settings }),
            })

            if (!response.ok) {
                throw new Error("Failed to generate preview")
            }

            const previewData = await response.json()
            setPreview(previewData as DownloadPreview)
            toast.success("Preview generated successfully")
        } catch (error) {
            console.error("Failed to generate preview:", error)
            toast.error("Failed to generate preview")
        } finally {
            setIsLoading(false)
        }
    }, [settings])

    // Poll for active job status
    useEffect(() => {
        if (!activeJob) return

        const pollInterval = setInterval(async () => {
            try {
                const response = await fetch("/api/v1/anime/download-all/status")
                if (response.ok) {
                    const raw = await response.json() as any
                    const statusData = raw?.data || raw
                    const job = statusData?.job as AllAnimeDownloadJob | undefined
                    const st = statusData?.status as string | undefined
                    if (!job || st === "idle" || st === "no active job") {
                        setActiveJob(null)
                    } else {
                        setActiveJob(job)
                    }
                }
            } catch (error) {
                console.error("Failed to poll job status:", error)
            }
        }, 5000) // Poll every 5 seconds

        return () => clearInterval(pollInterval)
    }, [activeJob])

    // Check for existing job on mount and load database stats
    useEffect(() => {
        const checkExistingJob = async () => {
            try {
                const response = await fetch("/api/v1/anime/download-all/status")
                if (response.ok) {
                    const result = await response.json() as any
                    const statusData = result.data || result
                    
                    // Extract database statistics from status response
                    if (statusData.totalAnime && statusData.totalAnime > 0) {
                        setDatabaseStats({
                            totalAnime: statusData.totalAnime,
                            estimatedSizeGB: statusData.estimatedSizeGB,
                            ...statusData.databaseStats
                        })
                        
                        // Mark database as loaded if we have anime count
                        if (!databaseStatus.loaded) {
                            setDatabaseStatus({
                                loaded: true,
                                totalAnime: statusData.totalAnime,
                                stats: {
                                    totalAnime: statusData.totalAnime,
                                    estimatedSizeGB: statusData.estimatedSizeGB,
                                    ...statusData.databaseStats
                                }
                            })
                        }
                    }
                    
                    // Check for active job
                    if (statusData.job && statusData.status !== "idle") {
                        setActiveJob(statusData.job as AllAnimeDownloadJob)
                    }
                }
            } catch (error) {
                console.error("Failed to check existing job:", error)
            }
        }

        checkExistingJob()
    }, [])

    // WebSocket event listeners
    useEffect(() => {
        const handleAllAnimeProgress = (data: AllAnimeDownloadJob) => {
            setActiveJob(data)
        }

        const handleAllAnimeComplete = (data: AllAnimeDownloadJob) => {
            setActiveJob(data)
            if (data.status === "completed") {
                toast.success(`All-anime download completed! ${data.completedAnime} anime downloaded successfully.`)
            } else if (data.status === "failed") {
                toast.error(`All-anime download failed: ${data.error}`)
            }
        }

        // Register WebSocket listeners (assuming these events exist)
        // if (sendMessage) {
        //     // This would depend on your WebSocket implementation
        //     // sendMessage({ type: "subscribe", event: "all_anime_download_progress" })
        //     // sendMessage({ type: "subscribe", event: "all_anime_download_complete" })
        // }

        return () => {
            // Cleanup WebSocket listeners
        }
    }, []) // Removed sendMessage dependency

    return {
        databaseStatus,
        activeJob,
        settings,
        preview,
        isLoading,
        databaseStats,
        loadDatabase,
        startDownload,
        cancelDownload,
        updateSettings,
        generatePreview,
        applyPreset,
        resetSettings,
        exportSettings,
        importSettings,
    }
}
