"use client"

import React from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ProgressBar } from "@/components/ui/progress-bar"
import { Badge } from "@/components/ui/badge"
import { 
    BiPlay, 
    BiPause, 
    BiStop, 
    BiDownload, 
    BiTrendingUp,
    BiError,
    BiCheckCircle,
    BiXCircle,
    BiTime
} from "react-icons/bi"
import { LuActivity } from "react-icons/lu"
import { AllAnimeDownloadJob } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderDashboardProps {
    databaseStats: any
    activeJob: AllAnimeDownloadJob | null
    onStartDownload: () => Promise<void>
    onCancelDownload: () => Promise<void>
    isLoading: boolean
}

export function AllAnimeDownloaderDashboard({
    databaseStats,
    activeJob,
    onStartDownload,
    onCancelDownload,
    isLoading,
}: AllAnimeDownloaderDashboardProps) {
    const getStatusIntent = (status: string) => {
        switch (status) {
            case "running":
                return "primary"
            case "completed":
                return "success"
            case "failed":
                return "alert"
            case "cancelled":
                return "gray"
            case "paused":
                return "warning"
            default:
                return "gray"
        }
    }

    const getStatusIcon = (status: string) => {
        switch (status) {
            case "running":
                return <LuActivity className="w-3 h-3 animate-pulse" />
            case "completed":
                return <BiDownload className="w-3 h-3" />
            case "failed":
                return <BiError className="w-3 h-3" />
            case "cancelled":
                return <BiStop className="w-3 h-3" />
            case "paused":
                return <BiPause className="w-3 h-3" />
            default:
                return <BiTime className="w-3 h-3" />
        }
    }

    const formatDuration = (startTime: string, endTime?: string) => {
        const start = new Date(startTime)
        const end = endTime ? new Date(endTime) : new Date()
        const duration = end.getTime() - start.getTime()
        
        const days = Math.floor(duration / (1000 * 60 * 60 * 24))
        const hours = Math.floor((duration % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
        const minutes = Math.floor((duration % (1000 * 60 * 60)) / (1000 * 60))
        
        if (days > 0) {
            return `${days}d ${hours}h ${minutes}m`
        } else if (hours > 0) {
            return `${hours}h ${minutes}m`
        } else {
            return `${minutes}m`
        }
    }

    const progressPercentage = activeJob?.progress || 0

    return (
        <div className="space-y-6">
            {/* Quick Stats */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <Card>
                    <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-sm font-medium text-muted-foreground">Total Anime</p>
                                <p className="text-2xl font-bold">
                                    {databaseStats?.totalAnime?.toLocaleString() || "0"}
                                </p>
                            </div>
                            <BiDownload className="w-8 h-8 text-muted-foreground" />
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-sm font-medium text-muted-foreground">Estimated Size</p>
                                <p className="text-2xl font-bold">
                                    ~{((databaseStats?.totalAnime || 0) * 8 / 1000).toFixed(0)}TB
                                </p>
                            </div>
                            <BiTrendingUp className="w-8 h-8 text-muted-foreground" />
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-sm font-medium text-muted-foreground">Active Batches</p>
                                <p className="text-2xl font-bold">
                                    {activeJob?.activeBatches || 0}
                                </p>
                            </div>
                            <LuActivity className="w-4 h-4 animate-pulse" />
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-sm font-medium text-muted-foreground">Status</p>
                                <Badge intent={getStatusIntent(activeJob?.status || "idle")} className="mt-1">
                                    {getStatusIcon(activeJob?.status || "idle")}
                                    <span className="ml-1 capitalize">
                                        {activeJob?.status || "idle"}
                                    </span>
                                </Badge>
                            </div>
                        </div>
                    </CardContent>
                </Card>
            </div>

            {/* Main Control Panel */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <BiDownload className="w-5 h-5" />
                        Download Control
                    </CardTitle>
                    <CardDescription>
                        Start, monitor, or cancel the all-anime batch download operation
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                    {!activeJob ? (
                        <div className="text-center space-y-4">
                            <div className="space-y-2">
                                <h3 className="text-lg font-semibold">Ready to Download All Anime</h3>
                                <p className="text-muted-foreground">
                                    This will start downloading every anime in the database with optimal torrent selection.
                                    Each anime will be downloaded as a separate batch with automatic linking.
                                </p>
                            </div>
                            <Button
                                onClick={onStartDownload}
                                disabled={isLoading}
                                size="lg"
                                className="w-full max-w-md"
                            >
                                <BiPlay className="w-4 h-4 mr-2" />
                                Start All-Anime Download
                            </Button>
                        </div>
                    ) : (
                        <div className="space-y-6">
                            {/* Progress Overview */}
                            <div className="space-y-4">
                                <div className="flex items-center justify-between">
                                    <h3 className="text-lg font-semibold">Download Progress</h3>
                                    <Badge intent={getStatusIntent(activeJob.status)}>
                                        {getStatusIcon(activeJob.status)}
                                        <span className="ml-1 capitalize">{activeJob.status}</span>
                                    </Badge>
                                </div>

                                <div className="space-y-2">
                                    <div className="flex justify-between text-sm">
                                        <span>Overall Progress</span>
                                        <span>{(activeJob?.progress ?? 0).toFixed(1)}%</span>
                                    </div>
                                    <ProgressBar value={progressPercentage} size="sm" />
                                </div>

                                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                                    <div className="space-y-1">
                                        <p className="text-muted-foreground">Total Anime</p>
                                        <p className="font-mono text-lg">{(activeJob?.totalAnime ?? 0).toLocaleString()}</p>
                                    </div>
                                    <div className="space-y-1">
                                        <p className="text-muted-foreground">Completed</p>
                                        <p className="font-mono text-lg text-green-600">{(activeJob?.completedAnime ?? 0).toLocaleString()}</p>
                                    </div>
                                    <div className="space-y-1">
                                        <p className="text-muted-foreground">Failed</p>
                                        <p className="font-mono text-lg text-red-600">{(activeJob?.failedAnime ?? 0).toLocaleString()}</p>
                                    </div>
                                    <div className="space-y-1">
                                        <p className="text-muted-foreground">Active</p>
                                        <p className="font-mono text-lg text-blue-600">{activeJob?.activeBatches ?? 0}</p>
                                    </div>
                                </div>

                                <div className="flex items-center justify-between pt-2 border-t">
                                    <div className="flex items-center gap-2">
                                        <BiCheckCircle className="w-4 h-4 text-green-500" />
                                        <span className="text-sm text-muted-foreground">
                                            Running for {formatDuration(activeJob.startTime, activeJob.endTime)}
                                        </span>
                                    </div>
                                    {activeJob.statistics?.estimatedTimeLeft && (
                                        <div className="text-sm text-muted-foreground">
                                            ETA: {activeJob?.statistics?.estimatedTimeLeft ?? "Calculating..."}
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Control Buttons */}
                            <div className="flex gap-2">
                                <Button
                                    intent="alert"
                                    onClick={onCancelDownload}
                                    disabled={isLoading || activeJob.status === "cancelled"}
                                >
                                    <BiPause className="w-4 h-4 mr-2" />
                                    Cancel Download
                                </Button>
                            </div>

                            {/* Error Display */}
                            {activeJob.error && (
                                <div className="p-4 bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded-lg">
                                    <div className="flex items-center gap-2 text-red-800 dark:text-red-200">
                                        <BiError className="w-4 h-4" />
                                        <span className="font-medium">Error</span>
                                    </div>
                                    <p className="text-sm text-red-700 dark:text-red-300 mt-1">
                                        {activeJob.error}
                                    </p>
                                </div>
                            )}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
