"use client"

import { 
    useGetNyaaCrawlerStatus, 
    useStartNyaaCrawler, 
    useStopNyaaCrawler
} from "@/api/hooks/nyaa-crawler.hooks"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import React from "react"
import { BiPlay, BiStop } from "react-icons/bi"

export function NyaaCrawlerScreen() {
    const { data: status, isLoading: statusLoading } = useGetNyaaCrawlerStatus()
    const startMutation = useStartNyaaCrawler()
    const stopMutation = useStopNyaaCrawler()

    const handleStart = () => {
        startMutation.mutate(undefined)
    }

    const handleStop = () => {
        stopMutation.mutate(undefined)
    }

    const formatTime = (timeString: string) => {
        if (!timeString) return "N/A"
        try {
            return new Date(timeString).toLocaleString()
        } catch {
            return "N/A"
        }
    }

    const getStatusColor = () => {
        if (statusLoading) return "text-muted-foreground"
        if (status?.isRunning) return "text-green-500"
        return "text-muted-foreground"
    }

    const getStatusText = () => {
        if (statusLoading) return "Loading..."
        if (status?.isRunning) return "Running"
        return "Stopped"
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-2xl font-bold">Nyaa Torrent Crawler</h2>
                    <p className="text-muted-foreground">
                        Batch download anime torrents from Nyaa.si using your Python crawler
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <span className={`text-sm font-medium ${getStatusColor()}`}>
                        {getStatusText()}
                    </span>
                    <div className="flex gap-2">
                        <Button
                            onClick={handleStart}
                            disabled={status?.isRunning || startMutation.isPending}
                            size="sm"
                            intent="primary"
                        >
                            <BiPlay className="w-4 h-4 mr-2" />
                            Start Crawler
                        </Button>
                        <Button
                            onClick={handleStop}
                            disabled={!status?.isRunning || stopMutation.isPending}
                            size="sm"
                            intent="alert"
                        >
                            <BiStop className="w-4 h-4 mr-2" />
                            Stop Crawler
                        </Button>
                    </div>
                </div>
            </div>

            <Separator />

            <div className="grid gap-6">
                <Card>
                    <CardHeader>
                        <CardTitle>Progress</CardTitle>
                        <CardDescription>
                            Current progress of the Nyaa crawler operation
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        {status && (
                            <>
                                <div className="space-y-2">
                                    <div className="flex justify-between text-sm">
                                        <span>Overall Progress</span>
                                        <span>{Math.round((status.progress || 0) * 100)}%</span>
                                    </div>
                                    <div className="w-full bg-gray-200 rounded-full h-2.5">
                                        <div 
                                            className="bg-blue-600 h-2.5 rounded-full transition-all duration-300" 
                                            style={{ width: `${(status.progress || 0) * 100}%` }}
                                        ></div>
                                    </div>
                                </div>
                                
                                <div className="grid grid-cols-2 gap-4 text-sm">
                                    <div>
                                        <span className="text-muted-foreground">Total Queries:</span>
                                        <div className="font-medium">{status.totalQueries || 0}</div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">Processed:</span>
                                        <div className="font-medium">{status.processedQueries || 0}</div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">Torrents Found:</span>
                                        <div className="font-medium">{status.torrentsFound || 0}</div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">Torrents Added:</span>
                                        <div className="font-medium">{status.torrentsAdded || 0}</div>
                                    </div>
                                </div>

                                {status.currentQuery && (
                                    <div className="pt-2 border-t">
                                        <span className="text-muted-foreground text-sm">Current Query:</span>
                                        <div className="font-medium text-sm mt-1">{status.currentQuery}</div>
                                    </div>
                                )}

                                {status.startTime && (
                                    <div className="pt-2 border-t">
                                        <span className="text-muted-foreground text-sm">Started At:</span>
                                        <div className="font-medium text-sm mt-1">{formatTime(status.startTime)}</div>
                                    </div>
                                )}
                            </>
                        )}

                        {!status && !statusLoading && (
                            <div className="text-center text-muted-foreground py-8">
                                No status information available
                            </div>
                        )}

                        {statusLoading && (
                            <div className="text-center text-muted-foreground py-8">
                                Loading status...
                            </div>
                        )}
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Information</CardTitle>
                        <CardDescription>
                            About the Nyaa Torrent Crawler
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3 text-sm">
                        <div>
                            <strong>What it does:</strong> Searches Nyaa.si for anime torrents using your configured search queries 
                            and automatically adds them to qBittorrent for download.
                        </div>
                        <div>
                            <strong>Processing:</strong> Queries are processed sequentially with configurable delays 
                            between requests to be respectful to Nyaa.si servers.
                        </div>
                        <div>
                            <strong>Integration:</strong> Uses your Python crawler script with qBittorrent integration 
                            for seamless torrent management.
                        </div>
                        <div className="text-amber-600">
                            <strong>Note:</strong> Make sure qBittorrent is running and accessible before starting the crawler.
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Crawler Logs</CardTitle>
                        <CardDescription>
                            Real-time logs from the crawler operation
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="bg-gray-900 text-green-400 p-4 rounded-md font-mono text-sm max-h-96 overflow-y-auto">
                            {status?.logs && status.logs.length > 0 ? (
                                status.logs.map((log, index) => (
                                    <div key={index} className="mb-1">
                                        {log}
                                    </div>
                                ))
                            ) : (
                                <div className="text-gray-500">No logs available</div>
                            )}
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    )
}
