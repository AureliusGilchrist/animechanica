"use client"

import { useGetEnMasseDownloaderStatus, useStartEnMasseDownloader, useStopEnMasseDownloader } from "@/api/hooks/en_masse_downloader.hooks"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
// import { cn } from "@/lib/utils" // Removed to fix import issue
import React from "react"
import { BiPlay, BiStop } from "react-icons/bi"
import { toast } from "sonner"
import { NyaaCrawlerScreen } from "../nyaa-crawler/nyaa-crawler-screen"

function MangaEnMasseDownloader() {
    const { data: status, isLoading } = useGetEnMasseDownloaderStatus()
    const startMutation = useStartEnMasseDownloader()
    const stopMutation = useStopEnMasseDownloader()
    
    const handleStart = () => {
        startMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Downloader started")
            },
            onError: (error) => {
                toast.error(`Failed to start En Masse Downloader: ${error.message}`)
            },
        })
    }

    const handleStop = () => {
        stopMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Downloader stopped")
            },
            onError: (error) => {
                toast.error(`Failed to stop En Masse Downloader: ${error.message}`)
            },
        })
    }

    const formatTime = (timeString: string) => {
        if (!timeString) return "N/A"
        try {
            return new Date(timeString).toLocaleString()
        } catch {
            return "N/A"
        }
    }

    const renderStatusBadge = () => {
        if (isLoading) return <Badge intent="gray" size="sm">Loading…</Badge>
        if (status?.isRunning) return <Badge intent="success" size="sm">Running</Badge>
        return <Badge intent="gray" size="sm">Stopped</Badge>
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-2xl font-bold">Manga En Masse Downloader</h2>
                    <p className="text-muted-foreground">
                        Download all manga series from the weebcentral catalogue
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    {renderStatusBadge()}
                    <div className="flex gap-2">
                        <Button
                            onClick={handleStart}
                            disabled={status?.isRunning || startMutation.isPending}
                            size="sm"
                            intent="primary"
                        >
                            <BiPlay className="w-4 h-4 mr-2" />
                            Start
                        </Button>
                        <Button
                            onClick={handleStop}
                            disabled={!status?.isRunning || stopMutation.isPending}
                            size="sm"
                            intent="alert"
                        >
                            <BiStop className="w-4 h-4 mr-2" />
                            Stop
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
                            Current progress of the en masse download operation
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        {status && (
                            <>
                                {(status as any)?.warmupActive && (
                                    <div className="space-y-2 p-3 rounded-md border border-blue-300 bg-blue-50 dark:bg-opacity-10">
                                        <div className="flex items-center justify-between text-sm">
                                            <div className="flex items-center gap-2">
                                                <Badge intent="info" size="sm">Warm-up</Badge>
                                                <span className="text-muted-foreground">Preparing popularity</span>
                                            </div>
                                            <span className="font-medium">{Math.round(((status as any)?.warmupPercent ?? 0) * 100)}%</span>
                                        </div>
                                        <div className="w-full bg-gray-200 rounded-full h-2.5">
                                            <div
                                                className="bg-blue-600 h-2.5 rounded-full transition-all duration-300"
                                                style={{ width: `${Math.round((((status as any)?.warmupPercent ?? 0) * 100))}%` }}
                                            />
                                        </div>
                                        {((status as any)?.warmupTopCandidate) && (
                                            <div className="text-xs text-blue-900 dark:text-blue-300">
                                                Top candidate so far: <span className="font-medium">{(status as any).warmupTopCandidate}</span>
                                            </div>
                                        )}
                                        <div className="text-xs text-blue-900 dark:text-blue-300">
                                            {`Warm-up: ${((status as any)?.warmupReady ?? 0).toLocaleString()}/${((status as any)?.warmupTarget ?? 0).toLocaleString()} (${Math.round((((status as any)?.warmupPercent ?? 0) * 100))}%)`}
                                            {((status as any)?.warmupTopCandidate) ? ` — Top: ${(status as any).warmupTopCandidate}` : ""}
                                        </div>
                                    </div>
                                )}
                                <div className="space-y-2">
                                    <div className="flex justify-between text-sm">
                                        <span>Overall Progress</span>
                                        <span>{Math.round(status.progress)}%</span>
                                    </div>
                                    <div className="w-full bg-gray-200 rounded-full h-2.5">
                                        <div 
                                            className="bg-blue-600 h-2.5 rounded-full transition-all duration-300" 
                                            style={{ width: `${status.progress}%` }}
                                        ></div>
                                    </div>
                                </div>
                                
                                <div className="grid grid-cols-2 gap-4 text-sm">
                                    <div>
                                        <span className="text-muted-foreground">Total Series:</span>
                                        <div className="font-medium">{status.totalSeries.toLocaleString()}</div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">Processed:</span>
                                        <div className="font-medium">{status.processedSeries.toLocaleString()}</div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">Remaining:</span>
                                        <div className="font-medium">
                                            {(status.totalSeries - status.processedSeries).toLocaleString()}
                                        </div>
                                    </div>
                                    <div>
                                        <span className="text-muted-foreground">ETA:</span>
                                        <div className="font-medium">{status.estimatedTimeRemaining || "N/A"}</div>
                                    </div>
                                </div>

                                {status.currentSeries && (
                                    <div className="pt-2 border-t">
                                        <span className="text-muted-foreground text-sm">Currently Processing:</span>
                                        <div className="font-medium text-sm mt-1">{status.currentSeries}</div>
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

                        {!status && !isLoading && (
                            <div className="text-center text-muted-foreground py-8">
                                No status information available
                            </div>
                        )}

                        {isLoading && (
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
                            About the En Masse Downloader
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3 text-sm">
                        <div>
                            <strong>What it does:</strong> Processes all manga series from the weebcentral catalogue 
                            and queues them for download using Seanime's existing manga download system.
                        </div>
                        <div>
                            <strong>Processing:</strong> Series are processed sequentially with a 3-second delay 
                            between each series and 100ms delay between chapters.
                        </div>
                        <div>
                            <strong>Integration:</strong> Downloaded manga will appear in your manga collection 
                            and can be accessed through the regular manga interface.
                        </div>
                        <div className="text-amber-600">
                            <strong>Note:</strong> This is a long-running process that may take several hours 
                            depending on the size of the catalogue. You can safely stop and restart it at any time.
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    )
}

export function EnMasseDownloader() {
    const [activeTab, setActiveTab] = React.useState("manga")
    
    React.useEffect(() => {
        // Check URL parameters for tab selection
        const urlParams = new URLSearchParams(window.location.search)
        const tabParam = urlParams.get("tab")
        if (tabParam === "anime") {
            setActiveTab("anime")
        }
    }, [])
    
    return (
        <div className="container mx-auto p-6 space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold">En Masse Downloader</h1>
                    <p className="text-muted-foreground">
                        Download all anime and manga series in bulk
                    </p>
                </div>
            </div>

            <Separator />

            <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
                <TabsList className="grid w-full grid-cols-2">
                    <TabsTrigger value="manga">Manga Downloader</TabsTrigger>
                    <TabsTrigger value="anime">Anime Downloader</TabsTrigger>
                </TabsList>
                
                <TabsContent value="manga" className="mt-6">
                    <MangaEnMasseDownloader />
                </TabsContent>
                
                <TabsContent value="anime" className="mt-6">
                    <NyaaCrawlerScreen />
                </TabsContent>
            </Tabs>
        </div>
    )
}
