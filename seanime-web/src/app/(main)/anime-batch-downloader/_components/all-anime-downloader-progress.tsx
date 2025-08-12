"use client"

import React from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ProgressBar } from "@/components/ui/progress-bar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { 
    BiTime, 
    BiDownload, 
    BiError,
    BiCheckCircle,
    BiXCircle,
    BiPause,
    BiStop,
    BiTrendingUp
} from "react-icons/bi"
import { LuActivity } from "react-icons/lu"
import { AllAnimeDownloadJob } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderProgressProps {
    job: AllAnimeDownloadJob
    onCancelDownload: () => Promise<void>
}

export function AllAnimeDownloaderProgress({
    job,
    onCancelDownload,
}: AllAnimeDownloaderProgressProps) {
    // Handle case where job is null or undefined
    if (!job) {
        return (
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <BiDownload className="w-5 h-5" />
                        Anime Batch Download Progress
                    </CardTitle>
                    <CardDescription>
                        No active download job
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="text-center text-muted-foreground py-8">
                        No anime batch download is currently running.
                    </div>
                </CardContent>
            </Card>
        )
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

    const getStatusIcon = (status: string) => {
        switch (status) {
            case "running":
                return <LuActivity className="w-4 h-4 animate-pulse" />
            case "completed":
                return <BiCheckCircle className="w-4 h-4 text-green-500" />
            case "failed":
                return <BiXCircle className="w-4 h-4 text-red-500" />
            case "cancelled":
                return <BiStop className="w-4 h-4 text-gray-500" />
            case "paused":
                return <BiPause className="w-4 h-4 text-yellow-500" />
            default:
                return <BiTime className="w-4 h-4" />
        }
    }

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

    const completionRate = (job?.totalAnime ?? 0) > 0 ? ((job?.completedAnime ?? 0) / (job?.totalAnime ?? 1)) * 100 : 0
    const failureRate = (job?.totalAnime ?? 0) > 0 ? ((job?.failedAnime ?? 0) / (job?.totalAnime ?? 1)) * 100 : 0
    const remainingAnime = (job?.totalAnime ?? 0) - (job?.completedAnime ?? 0) - (job?.failedAnime ?? 0)

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <BiDownload className="w-5 h-5" />
                    Download Progress
                </CardTitle>
                <CardDescription>
                    Real-time progress of the all-anime batch download operation
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Status Header */}
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <Badge intent={getStatusIntent(job.status)} className="text-sm">
                            {getStatusIcon(job.status)}
                            <span className="ml-2 capitalize">{job.status}</span>
                        </Badge>
                        <div className="text-sm text-muted-foreground">
                            Job ID: <code className="text-xs bg-gray-100 dark:bg-gray-800 px-1 rounded">{job.id}</code>
                        </div>
                    </div>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <BiTime className="w-4 h-4" />
                        <span>Running for {formatDuration(job.startTime, job.endTime)}</span>
                    </div>
                </div>

                {/* Main Progress Bar */}
                <div className="space-y-3">
                    <div className="flex justify-between items-center">
                        <h3 className="text-lg font-semibold">Overall Progress</h3>
                        <span className="text-2xl font-bold">{(job?.progress ?? 0).toFixed(1)}%</span>
                    </div>
                    <ProgressBar value={job?.progress ?? 0} size="sm" />
                    <div className="flex justify-between text-sm text-muted-foreground">
                        <span>{(job?.completedAnime ?? 0) + (job?.failedAnime ?? 0)} of {job?.totalAnime ?? 0} processed</span>
                        <span>{remainingAnime} remaining</span>
                    </div>
                </div>

                {/* Detailed Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <div className="text-center p-4 bg-green-50 dark:bg-green-950 rounded-lg">
                        <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                            {(job?.completedAnime ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-green-700 dark:text-green-300">Completed</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            {completionRate.toFixed(1)}%
                        </div>
                    </div>

                    <div className="text-center p-4 bg-red-50 dark:bg-red-950 rounded-lg">
                        <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                            {(job?.failedAnime ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-red-700 dark:text-red-300">Failed</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            {failureRate.toFixed(1)}%
                        </div>
                    </div>

                    <div className="text-center p-4 bg-blue-50 dark:bg-blue-950 rounded-lg">
                        <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                            {job.activeBatches}
                        </div>
                        <div className="text-sm text-blue-700 dark:text-blue-300">Active</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            Batches
                        </div>
                    </div>

                    <div className="text-center p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                        <div className="text-2xl font-bold text-gray-600 dark:text-gray-400">
                            {remainingAnime.toLocaleString()}
                        </div>
                        <div className="text-sm text-gray-700 dark:text-gray-300">Remaining</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            To Process
                        </div>
                    </div>
                </div>

                {/* Performance Metrics */}
                {job.statistics && (
                    <div className="space-y-4">
                        <h3 className="text-lg font-semibold flex items-center gap-2">
                            <BiTrendingUp className="w-4 h-4" />
                            Performance Metrics
                        </h3>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div className="space-y-2">
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">Data Downloaded</span>
                                    <span className="text-sm font-mono">
                                        {(job?.statistics?.downloadedSizeGb ?? 0).toFixed(1)} GB
                                    </span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">Total Size</span>
                                    <span className="text-sm font-mono">
                                        {(job?.statistics?.totalSizeGb ?? 0).toFixed(1)} GB
                                    </span>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">Torrents Added</span>
                                    <span className="text-sm font-mono">
                                        {(job?.statistics?.torrentsAdded ?? 0).toLocaleString()}
                                    </span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">qBittorrent Active</span>
                                    <span className="text-sm font-mono">
                                        {job?.statistics?.qbittorrentActive ?? 0}
                                    </span>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">ETA</span>
                                    <span className="text-sm font-mono">
                                        {job?.statistics?.estimatedTimeLeft ?? "Calculating..."}
                                    </span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-sm text-muted-foreground">Avg Speed</span>
                                    <span className="text-sm font-mono">
                                        {((job?.statistics?.averageSpeed ?? 0) / 1024 / 1024).toFixed(1)} MB/s
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* Quality Achievement Stats */}
                {job.statistics && (
                    <div className="space-y-4">
                        <h3 className="text-lg font-semibold">Quality Achievement</h3>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div className="flex items-center justify-between p-3 bg-purple-50 dark:bg-purple-950 rounded-lg">
                                <div className="flex items-center gap-2">
                                    <div className="w-3 h-3 bg-purple-500 rounded-full"></div>
                                    <span className="text-sm">Dual Audio</span>
                                </div>
                                <span className="font-mono text-sm">
                                    {(job?.statistics?.dualAudioCount ?? 0).toLocaleString()}
                                </span>
                            </div>

                            <div className="flex items-center justify-between p-3 bg-indigo-50 dark:bg-indigo-950 rounded-lg">
                                <div className="flex items-center gap-2">
                                    <div className="w-3 h-3 bg-indigo-500 rounded-full"></div>
                                    <span className="text-sm">Bluray/BD</span>
                                </div>
                                <span className="font-mono text-sm">
                                    {(job?.statistics?.blurayCount ?? 0).toLocaleString()}
                                </span>
                            </div>

                            <div className="flex items-center justify-between p-3 bg-cyan-50 dark:bg-cyan-950 rounded-lg">
                                <div className="flex items-center gap-2">
                                    <div className="w-3 h-3 bg-cyan-500 rounded-full"></div>
                                    <span className="text-sm">High Resolution</span>
                                </div>
                                <span className="font-mono text-sm">
                                    {(job?.statistics?.highResCount ?? 0).toLocaleString()}
                                </span>
                            </div>
                        </div>
                    </div>
                )}

                {/* Error Display */}
                {job.error && (
                    <div className="p-4 bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded-lg">
                        <div className="flex items-center gap-2 text-red-800 dark:text-red-200 mb-2">
                            <BiError className="w-4 h-4" />
                            <span className="font-medium">Error Details</span>
                        </div>
                        <p className="text-sm text-red-700 dark:text-red-300">
                            {job.error}
                        </p>
                    </div>
                )}

                {/* Control Actions */}
                {job.status === "running" && (
                    <div className="pt-4 border-t">
                        <Button
                            intent="alert"
                            onClick={onCancelDownload}
                            className="w-full"
                        >
                            <BiStop className="w-4 h-4 mr-2" />
                            Cancel All-Anime Download
                        </Button>
                        <p className="text-xs text-muted-foreground mt-2 text-center">
                            This will stop all active downloads and cancel the operation
                        </p>
                    </div>
                )}
            </CardContent>
        </Card>
    )
}
