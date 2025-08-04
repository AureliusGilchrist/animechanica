"use client"
import { useGetEnMasseStatus, useStartEnMasseDownload, usePauseEnMasseDownload, useResumeEnMasseDownload, useStopEnMasseDownload } from "@/api/hooks/manga_enmasse.hooks"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Separator } from "@/components/ui/separator"
import { toast } from "sonner"
import React from "react"
import { BiPlay, BiPause, BiStop, BiRefresh } from "react-icons/bi"
import { FaBookReader } from "react-icons/fa"

export default function EnMasseDownloaderPage() {
    const { data: status, isLoading, refetch } = useGetEnMasseStatus()
    const startMutation = useStartEnMasseDownload()
    const pauseMutation = usePauseEnMasseDownload()
    const resumeMutation = useResumeEnMasseDownload()
    const stopMutation = useStopEnMasseDownload()

    const handleStart = () => {
        startMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Download started!")
                refetch()
            },
            onError: (error: any) => {
                toast.error(`Failed to start: ${error.message}`)
            }
        })
    }

    const handlePause = () => {
        pauseMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Download paused")
                refetch()
            },
            onError: (error: any) => {
                toast.error(`Failed to pause: ${error.message}`)
            }
        })
    }

    const handleResume = () => {
        resumeMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Download resumed")
                refetch()
            },
            onError: (error: any) => {
                toast.error(`Failed to resume: ${error.message}`)
            }
        })
    }

    const handleStop = () => {
        stopMutation.mutate(undefined, {
            onSuccess: () => {
                toast.success("En Masse Download stopped")
                refetch()
            },
            onError: (error: any) => {
                toast.error(`Failed to stop: ${error.message}`)
            }
        })
    }

    if (isLoading) {
        return (
            <PageWrapper className="p-4 space-y-4">
                <div className="flex items-center gap-2">
                    <FaBookReader className="text-2xl" />
                    <h1 className="text-3xl font-bold">En Masse Downloader</h1>
                </div>
                <LoadingSpinner />
            </PageWrapper>
        )
    }

    const getStatusBadge = () => {
        if (!status) return null
        
        switch (status.status) {
            case "running":
                return <Badge intent="success">Running</Badge>
            case "paused":
                return <Badge intent="warning">Paused</Badge>
            case "stopped":
                return <Badge intent="gray">Stopped</Badge>
            case "completed":
                return <Badge intent="info">Completed</Badge>
            case "error":
                return <Badge intent="alert">Error</Badge>
            default:
                return <Badge intent="gray">Idle</Badge>
        }
    }

    return (
        <PageWrapper className="p-4 space-y-6">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <FaBookReader className="text-2xl" />
                    <h1 className="text-3xl font-bold">En Masse Downloader</h1>
                    {getStatusBadge()}
                </div>
                <Button
                    onClick={() => refetch()}
                    intent="gray-outline"
                    size="sm"
                    leftIcon={<BiRefresh />}
                >
                    Refresh
                </Button>
            </div>

            <Card>
                <CardHeader>
                    <CardTitle>WeebCentral Manga Collection</CardTitle>
                    <CardDescription>
                        Download all manga from the WeebCentral catalogue using Kitsu API for metadata.
                        Each series will be processed sequentially with proper rate limiting.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    {/* Control Buttons */}
                    <div className="flex gap-2">
                        {!status?.isRunning ? (
                            <Button
                                onClick={handleStart}
                                leftIcon={<BiPlay />}
                                intent="success"
                                loading={startMutation.isPending}
                            >
                                Start Download
                            </Button>
                        ) : (
                            <>
                                {status.isPaused ? (
                                    <Button
                                        onClick={handleResume}
                                        leftIcon={<BiPlay />}
                                        intent="success"
                                        loading={resumeMutation.isPending}
                                    >
                                        Resume
                                    </Button>
                                ) : (
                                    <Button
                                        onClick={handlePause}
                                        leftIcon={<BiPause />}
                                        intent="warning"
                                        loading={pauseMutation.isPending}
                                    >
                                        Pause
                                    </Button>
                                )}
                                <Button
                                    onClick={handleStop}
                                    leftIcon={<BiStop />}
                                    intent="alert"
                                    loading={stopMutation.isPending}
                                >
                                    Stop
                                </Button>
                            </>
                        )}
                    </div>

                    <Separator />

                    {/* Status Information */}
                    {status && (
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div className="text-center">
                                <div className="text-2xl font-bold text-green-500">
                                    {status.processedCount || 0}
                                </div>
                                <div className="text-sm text-muted-foreground">Processed</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-blue-500">
                                    {status.totalCount || 0}
                                </div>
                                <div className="text-sm text-muted-foreground">Total</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-red-500">
                                    {status.errorCount || 0}
                                </div>
                                <div className="text-sm text-muted-foreground">Errors</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-purple-500">
                                    {status.totalCount > 0 
                                        ? Math.round((status.processedCount / status.totalCount) * 100)
                                        : 0}%
                                </div>
                                <div className="text-sm text-muted-foreground">Progress</div>
                            </div>
                        </div>
                    )}

                    {/* Current Series */}
                    {status?.currentSeries && (
                        <div className="bg-muted p-3 rounded-md">
                            <div className="text-sm text-muted-foreground">Currently processing:</div>
                            <div className="font-medium">{status.currentSeries}</div>
                        </div>
                    )}

                    {/* Error List */}
                    {status?.errorSeries && status.errorSeries.length > 0 && (
                        <div className="space-y-2">
                            <h3 className="font-semibold text-red-500">Failed Series:</h3>
                            <div className="max-h-40 overflow-y-auto space-y-1">
                                {status.errorSeries.map((error: string, index: number) => (
                                    <div key={index} className="text-sm bg-red-50 dark:bg-red-900/20 p-2 rounded text-red-700 dark:text-red-300">
                                        {error}
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* Information Card */}
            <Card>
                <CardHeader>
                    <CardTitle>How it works</CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 text-sm text-muted-foreground">
                    <p>• Reads the WeebCentral manga catalogue from the server</p>
                    <p>• Searches Kitsu API for each manga title to get metadata</p>
                    <p>• Assigns negative media IDs to distinguish from AniList manga</p>
                    <p>• Processes series sequentially with 3-second delays between each</p>
                    <p>• Respects Kitsu API rate limits (2-second delays between API calls)</p>
                    <p>• Manga will appear in your library with Kitsu metadata and images</p>
                </CardContent>
            </Card>
        </PageWrapper>
    )
}
