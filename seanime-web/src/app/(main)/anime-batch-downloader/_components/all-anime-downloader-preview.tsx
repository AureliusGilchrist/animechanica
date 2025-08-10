"use client"

import React from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ProgressBar } from "@/components/ui/progress-bar"
import { 
    BiShow, 
    BiPlay, 
    BiHdd, 
    BiTime,
    BiFilter,
    BiPieChart,
    BiCalendar,
    BiTrendingUp,
    BiError
} from "react-icons/bi"
import { DownloadPreview, AllAnimeDownloadSettings } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderPreviewProps {
    preview: DownloadPreview
    onStartDownload: (settings?: Partial<AllAnimeDownloadSettings>) => Promise<void>
    isLoading: boolean
}

export function AllAnimeDownloaderPreview({
    preview,
    onStartDownload,
    isLoading,
}: AllAnimeDownloaderPreviewProps) {
    const filteredPercentage = ((preview?.filtered ?? 0) / (preview?.totalInDatabase ?? 1)) * 100
    const downloadPercentage = ((preview?.willDownload ?? 0) / (preview?.totalInDatabase ?? 1)) * 100

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <BiShow className="w-5 h-5" />
                    Download Preview
                </CardTitle>
                <CardDescription>
                    Preview of what will be downloaded with your current settings
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Overview Stats */}
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                    <div className="text-center p-4 bg-blue-50 dark:bg-blue-950 rounded-lg">
                        <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                            {(preview?.totalInDatabase ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-blue-700 dark:text-blue-300">Total in Database</div>
                    </div>

                    <div className="text-center p-4 bg-green-50 dark:bg-green-950 rounded-lg">
                        <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                            {(preview?.willDownload ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-green-700 dark:text-green-300">Will Download</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            {downloadPercentage.toFixed(1)}%
                        </div>
                    </div>

                    <div className="text-center p-4 bg-red-50 dark:bg-red-950 rounded-lg">
                        <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                            {(preview?.filtered ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-red-700 dark:text-red-300">Filtered Out</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            {filteredPercentage.toFixed(1)}%
                        </div>
                    </div>

                    <div className="text-center p-4 bg-purple-50 dark:bg-purple-950 rounded-lg">
                        <div className="text-2xl font-bold text-purple-600 dark:text-purple-400">
                            ~{(preview?.estimatedSizeGB ?? 0).toLocaleString()}
                        </div>
                        <div className="text-sm text-purple-700 dark:text-purple-300">GB Estimated</div>
                        <div className="text-xs text-muted-foreground mt-1">
                            ~{(preview.estimatedSizeGB / 1024).toFixed(1)} TB
                        </div>
                    </div>
                </div>

                {/* Visual Progress Bar */}
                <div className="space-y-3">
                    <div className="flex justify-between items-center">
                        <h3 className="text-lg font-semibold">Database Coverage</h3>
                        <span className="text-sm text-muted-foreground">
                            {(preview?.willDownload ?? 0).toLocaleString()} of {(preview?.totalInDatabase ?? 0).toLocaleString()} anime
                        </span>
                    </div>
                    <div className="relative">
                        <ProgressBar value={downloadPercentage} size="md" />
                        <div className="absolute inset-0 flex items-center justify-center text-xs font-medium text-white mix-blend-difference">
                            {downloadPercentage.toFixed(1)}% will be downloaded
                        </div>
                    </div>
                </div>

                {/* Estimated Time and Size */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="flex items-center gap-3 p-4 bg-orange-50 dark:bg-orange-950 rounded-lg">
                        <BiTime className="w-8 h-8 text-orange-600 dark:text-orange-400" />
                        <div>
                            <div className="font-semibold text-orange-800 dark:text-orange-200">
                                Estimated Duration
                            </div>
                            <div className="text-lg font-mono text-orange-600 dark:text-orange-400">
                                {preview?.estimatedDuration ?? 'Unknown'}
                            </div>
                            <div className="text-xs text-orange-700 dark:text-orange-300">
                                This is a rough estimate
                            </div>
                        </div>
                    </div>

                    <div className="flex items-center gap-3 p-4 bg-indigo-50 dark:bg-indigo-950 rounded-lg">
                        <BiHdd className="w-8 h-8 text-indigo-600 dark:text-indigo-400" />
                        <div>
                            <div className="font-semibold text-indigo-800 dark:text-indigo-200">
                                Storage Required
                            </div>
                            <div className="text-lg font-mono text-indigo-600 dark:text-indigo-400">
                                ~{((preview?.estimatedSizeGB ?? 0) / 1024).toFixed(1)} TB
                            </div>
                            <div className="text-xs text-indigo-700 dark:text-indigo-300">
                                {(preview?.estimatedSizeGB ?? 0).toLocaleString()} GB total
                            </div>
                        </div>
                    </div>
                </div>

                {/* Quality Preferences Summary */}
                <div className="space-y-3">
                    <h3 className="text-lg font-semibold flex items-center gap-2">
                        <BiTrendingUp className="w-4 h-4" />
                        Quality Preferences
                    </h3>
                    <div className="flex flex-wrap gap-2">
                        {preview?.preferences?.dualAudio && (
                            <Badge intent="primary" className="bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">
                                Dual Audio Priority
                            </Badge>
                        )}
                        {preview?.preferences?.bluray && (
                            <Badge intent="primary" className="bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200">
                                Bluray/BD Priority
                            </Badge>
                        )}
                        {preview?.preferences?.highestRes && (
                            <Badge intent="primary" className="bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200">
                                Highest Resolution
                            </Badge>
                        )}
                    </div>
                </div>

                {/* Type Breakdown */}
                {Object.keys(preview?.typeBreakdown ?? {}).length > 0 && (
                    <div className="space-y-3">
                        <h3 className="text-lg font-semibold flex items-center gap-2">
                            <BiPieChart className="w-4 h-4" />
                            Content Type Breakdown
                        </h3>
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                            {Object.entries(preview?.typeBreakdown ?? {})
                                .sort(([,a], [,b]) => b - a)
                                .map(([type, count]) => (
                                    <div key={type} className="text-center p-3 border rounded-lg">
                                        <div className="text-lg font-bold">{(count ?? 0).toLocaleString()}</div>
                                        <div className="text-sm text-muted-foreground capitalize">{type}</div>
                                        <div className="text-xs text-muted-foreground">
                                            {((count / (preview?.willDownload ?? 1)) * 100).toFixed(1)}%
                                        </div>
                                    </div>
                                ))}
                        </div>
                    </div>
                )}

                {/* Year Breakdown */}
                {Object.keys(preview?.yearBreakdown ?? {}).length > 0 && (
                    <div className="space-y-3">
                        <h3 className="text-lg font-semibold flex items-center gap-2">
                            <BiCalendar className="w-4 h-4" />
                            Year Range Breakdown
                        </h3>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                            {Object.entries(preview?.yearBreakdown ?? {})
                                .sort(([a], [b]) => a.localeCompare(b))
                                .map(([yearRange, count]) => (
                                    <div key={yearRange} className="text-center p-3 border rounded-lg">
                                        <div className="text-lg font-bold">{(count ?? 0).toLocaleString()}</div>
                                        <div className="text-sm text-muted-foreground">{yearRange}</div>
                                        <div className="text-xs text-muted-foreground">
                                            {((count / (preview?.willDownload ?? 1)) * 100).toFixed(1)}%
                                        </div>
                                    </div>
                                ))}
                        </div>
                    </div>
                )}

                {/* Settings Applied */}
                <div className="space-y-3">
                    <h3 className="text-lg font-semibold flex items-center gap-2">
                        <BiFilter className="w-4 h-4" />
                        Applied Filters
                    </h3>
                    <div className="p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                            <div className="space-y-2">
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Year Range:</span>
                                    <span className="font-mono">
                                        {preview?.settings?.minYear ?? 'N/A'} - {preview?.settings?.maxYear ?? 'N/A'}
                                    </span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Min Seeders:</span>
                                    <span className="font-mono">{preview?.settings?.minSeeders ?? 0}</span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Max Concurrent:</span>
                                    <span className="font-mono">{preview?.settings?.maxConcurrentBatches ?? 0}</span>
                                </div>
                            </div>
                            <div className="space-y-2">
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Skip OVAs:</span>
                                    <Badge intent={preview?.settings?.skipOva ? "alert" : "gray"}>
                                        {preview?.settings?.skipOva ? "Yes" : "No"}
                                    </Badge>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Skip Specials:</span>
                                    <Badge intent={preview?.settings?.skipSpecials ? "alert" : "gray"}>
                                        {preview?.settings?.skipSpecials ? "Yes" : "No"}
                                    </Badge>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Warning for Large Downloads */}
                {(preview?.willDownload ?? 0) > 10000 && (
                    <div className="p-4 bg-yellow-50 dark:bg-yellow-950 border border-yellow-200 dark:border-yellow-800 rounded-lg">
                        <div className="flex items-center gap-2 text-yellow-800 dark:text-yellow-200 mb-2">
                            <BiError className="w-4 h-4" />
                            <span className="font-medium">Large Download Warning</span>
                        </div>
                        <p className="text-sm text-yellow-700 dark:text-yellow-300">
                            You're about to download {(preview?.willDownload ?? 0).toLocaleString()} anime titles. 
                            This will require approximately {(preview.estimatedSizeGB / 1024).toFixed(1)} TB of storage 
                            and will take {preview.estimatedDuration} to complete. Make sure you have sufficient 
                            storage space and bandwidth.
                        </p>
                    </div>
                )}

                {/* Start Download Button */}
                <div className="pt-4 border-t">
                    <Button
                        onClick={() => onStartDownload()}
                        disabled={isLoading || preview.willDownload === 0}
                        size="lg"
                        className="w-full"
                    >
                        <BiPlay className="w-4 h-4 mr-2" />
                        Start Download ({(preview?.willDownload ?? 0).toLocaleString()} Anime)
                    </Button>
                    <p className="text-xs text-muted-foreground mt-2 text-center">
                        This will start downloading all {(preview?.willDownload ?? 0).toLocaleString()} anime with your selected preferences
                    </p>
                </div>
            </CardContent>
        </Card>
    )
}
