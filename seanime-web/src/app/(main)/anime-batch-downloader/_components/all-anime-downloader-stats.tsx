"use client"

import React from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ProgressBar } from "@/components/ui/progress-bar"
import { Badge } from "@/components/ui/badge"
import { 
    BiBarChart, 
    BiHdd, 
    BiTrendingUp,
    BiVolumeMute,
    BiDisc,
    BiDesktop,
    BiDownload
} from "react-icons/bi"
import { LuActivity } from "react-icons/lu"
import { AllAnimeDownloadStats } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderStatsProps {
    statistics: AllAnimeDownloadStats
}

export function AllAnimeDownloaderStats({
    statistics,
}: AllAnimeDownloaderStatsProps) {
    const formatBytes = (bytes: number) => {
        if (bytes === 0) return "0 B"
        const k = 1024
        const sizes = ["B", "KB", "MB", "GB", "TB"]
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i]
    }

    const downloadProgress = statistics.totalSizeGb > 0 
        ? (statistics.downloadedSizeGb / statistics.totalSizeGb) * 100 
        : 0

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <BiBarChart className="w-5 h-5" />
                    Download Statistics
                </CardTitle>
                <CardDescription>
                    Detailed statistics and quality metrics for the all-anime download operation
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Storage Statistics */}
                <div className="space-y-4">
                    <h3 className="text-lg font-semibold flex items-center gap-2">
                        <BiHdd className="w-4 h-4" />
                        Storage & Transfer
                    </h3>
                    
                    <div className="space-y-3">
                        <div className="flex justify-between items-center">
                            <span className="text-sm font-medium">Download Progress</span>
                            <span className="text-sm font-mono">
                                {(statistics?.downloadedSizeGb ?? 0).toFixed(1)} GB / {(statistics?.totalSizeGb ?? 0).toFixed(1)} GB
                            </span>
                        </div>
                        <ProgressBar value={downloadProgress} size="sm" />
                        <div className="flex justify-between text-xs text-muted-foreground">
                            <span>{downloadProgress.toFixed(1)}% completed</span>
                            <span>{(statistics.totalSizeGb - statistics.downloadedSizeGb).toFixed(1)} GB remaining</span>
                        </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="text-center p-4 bg-blue-50 dark:bg-blue-950 rounded-lg">
                            <div className="text-xl font-bold text-blue-600 dark:text-blue-400">
                                {(statistics.averageSpeed / 1024 / 1024).toFixed(1)}
                            </div>
                            <div className="text-sm text-blue-700 dark:text-blue-300">MB/s</div>
                            <div className="text-xs text-muted-foreground">Avg Speed</div>
                        </div>

                        <div className="text-center p-4 bg-green-50 dark:bg-green-950 rounded-lg">
                            <div className="text-xl font-bold text-green-600 dark:text-green-400">
                                {(statistics?.torrentsAdded ?? 0).toLocaleString()}
                            </div>
                            <div className="text-sm text-green-700 dark:text-green-300">Torrents</div>
                            <div className="text-xs text-muted-foreground">Added</div>
                        </div>

                        <div className="text-center p-4 bg-orange-50 dark:bg-orange-950 rounded-lg">
                            <div className="text-xl font-bold text-orange-600 dark:text-orange-400">
                                {statistics.qbittorrentActive}
                            </div>
                            <div className="text-sm text-orange-700 dark:text-orange-300">Active</div>
                            <div className="text-xs text-muted-foreground">in qBittorrent</div>
                        </div>
                    </div>

                    {statistics.estimatedTimeLeft && (
                        <div className="p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                            <div className="flex items-center justify-between">
                                <div className="flex items-center gap-2">
                                    <BiTrendingUp className="w-4 h-4 text-muted-foreground" />
                                    <span className="font-medium">Estimated Time Remaining</span>
                                </div>
                                <span className="text-lg font-mono">
                                    {statistics.estimatedTimeLeft}
                                </span>
                            </div>
                        </div>
                    )}
                </div>

                {/* Quality Achievement Statistics */}
                <div className="space-y-4">
                    <h3 className="text-lg font-semibold">Quality Achievement Metrics</h3>
                    
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        {/* Dual Audio Achievement */}
                        <Card className="bg-purple-50 dark:bg-purple-950 border-purple-200 dark:border-purple-800">
                            <CardContent className="p-4">
                                <div className="flex items-center justify-between mb-3">
                                    <div className="flex items-center gap-2">
                                        <BiVolumeMute className="w-4 h-4 text-purple-600" />
                                        <span className="font-medium text-purple-800 dark:text-purple-200">
                                            Dual Audio
                                        </span>
                                    </div>
                                    <Badge className="bg-purple-100 dark:bg-purple-900">
                                        Priority #1
                                    </Badge>
                                </div>
                                <div className="text-center">
                                    <div className="text-2xl font-bold text-purple-600 dark:text-purple-400">
                                        {(statistics?.dualAudioCount ?? 0).toLocaleString()}
                                    </div>
                                    <div className="text-sm text-purple-700 dark:text-purple-300">
                                        Anime with dual audio
                                    </div>
                                    <div className="text-xs text-muted-foreground mt-1">
                                        {statistics.torrentsAdded > 0 
                                            ? ((statistics.dualAudioCount / statistics.torrentsAdded) * 100).toFixed(1)
                                            : 0}% achievement rate
                                    </div>
                                </div>
                            </CardContent>
                        </Card>

                        {/* Bluray Achievement */}
                        <Card className="bg-indigo-50 dark:bg-indigo-950 border-indigo-200 dark:border-indigo-800">
                            <CardContent className="p-4">
                                <div className="flex items-center justify-between mb-3">
                                    <div className="flex items-center gap-2">
                                        <BiDisc className="w-4 h-4 text-indigo-600" />
                                        <span className="font-medium text-indigo-800 dark:text-indigo-200">
                                            Bluray/BD
                                        </span>
                                    </div>
                                    <Badge className="bg-indigo-100 dark:bg-indigo-900">
                                        Priority #2
                                    </Badge>
                                </div>
                                <div className="text-center">
                                    <div className="text-2xl font-bold text-indigo-600 dark:text-indigo-400">
                                        {(statistics?.blurayCount ?? 0).toLocaleString()}
                                    </div>
                                    <div className="text-sm text-indigo-700 dark:text-indigo-300">
                                        Bluray/BD releases
                                    </div>
                                    <div className="text-xs text-muted-foreground mt-1">
                                        {statistics.torrentsAdded > 0 
                                            ? ((statistics.blurayCount / statistics.torrentsAdded) * 100).toFixed(1)
                                            : 0}% achievement rate
                                    </div>
                                </div>
                            </CardContent>
                        </Card>

                        {/* High Resolution Achievement */}
                        <Card className="bg-cyan-50 dark:bg-cyan-950 border-cyan-200 dark:border-cyan-800">
                            <CardContent className="p-4">
                                <div className="flex items-center justify-between mb-3">
                                    <div className="flex items-center gap-2">
                                        <BiDesktop className="w-4 h-4 text-cyan-600" />
                                        <span className="font-medium text-cyan-800 dark:text-cyan-200">
                                            High Resolution
                                        </span>
                                    </div>
                                    <Badge className="bg-cyan-100 dark:bg-cyan-900">
                                        Priority #4
                                    </Badge>
                                </div>
                                <div className="text-center">
                                    <div className="text-2xl font-bold text-cyan-600 dark:text-cyan-400">
                                        {(statistics?.highResCount ?? 0).toLocaleString()}
                                    </div>
                                    <div className="text-sm text-cyan-700 dark:text-cyan-300">
                                        1080p+ releases
                                    </div>
                                    <div className="text-xs text-muted-foreground mt-1">
                                        {statistics.torrentsAdded > 0 
                                            ? ((statistics.highResCount / statistics.torrentsAdded) * 100).toFixed(1)
                                            : 0}% achievement rate
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </div>

                {/* Performance Breakdown */}
                <div className="space-y-4">
                    <h3 className="text-lg font-semibold flex items-center gap-2">
                        <LuActivity className="w-4 h-4" />
                        Performance Breakdown
                    </h3>
                    
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="text-center p-3 border rounded-lg">
                            <div className="text-lg font-bold">
                                {(statistics.totalSizeGb / 1024).toFixed(1)}
                            </div>
                            <div className="text-sm text-muted-foreground">TB Total</div>
                        </div>
                        
                        <div className="text-center p-3 border rounded-lg">
                            <div className="text-lg font-bold">
                                {(statistics.downloadedSizeGb / 1024).toFixed(1)}
                            </div>
                            <div className="text-sm text-muted-foreground">TB Downloaded</div>
                        </div>
                        
                        <div className="text-center p-3 border rounded-lg">
                            <div className="text-lg font-bold">
                                {statistics.torrentsAdded > 0 
                                    ? (statistics.totalSizeGb / statistics.torrentsAdded).toFixed(1)
                                    : "0"}
                            </div>
                            <div className="text-sm text-muted-foreground">GB per Anime</div>
                        </div>
                        
                        <div className="text-center p-3 border rounded-lg">
                            <div className="text-lg font-bold">
                                {downloadProgress.toFixed(1)}%
                            </div>
                            <div className="text-sm text-muted-foreground">Complete</div>
                        </div>
                    </div>
                </div>

                {/* Quality Summary */}
                <div className="p-4 bg-gradient-to-r from-blue-50 to-purple-50 dark:from-blue-950 dark:to-purple-950 rounded-lg border">
                    <h4 className="font-medium mb-3 flex items-center gap-2">
                        <BiDownload className="w-4 h-4" />
                        Quality Summary
                    </h4>
                    <div className="text-sm space-y-2">
                        <div className="flex justify-between">
                            <span>Optimal Quality Achieved:</span>
                            <span className="font-mono">
                                {statistics.torrentsAdded > 0 
                                    ? (((statistics.dualAudioCount + statistics.blurayCount + statistics.highResCount) / (statistics.torrentsAdded * 3)) * 100).toFixed(1)
                                    : 0}%
                            </span>
                        </div>
                        <div className="text-xs text-muted-foreground">
                            Based on achieving dual audio, bluray quality, and high resolution preferences
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    )
}
