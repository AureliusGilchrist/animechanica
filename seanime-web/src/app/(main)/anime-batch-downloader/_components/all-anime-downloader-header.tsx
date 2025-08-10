"use client"

import React from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { BiData, BiCheckCircle, BiXCircle, BiRefresh } from "react-icons/bi"
import { LuLoader } from "react-icons/lu"
import { DatabaseStatus } from "../_lib/use-all-anime-downloader"

interface AllAnimeDownloaderHeaderProps {
    databaseStatus: DatabaseStatus
    onLoadDatabase: (path?: string) => Promise<void>
    isLoading: boolean
}

export function AllAnimeDownloaderHeader({
    databaseStatus,
    onLoadDatabase,
    isLoading,
}: AllAnimeDownloaderHeaderProps) {
    const [databasePath, setDatabasePath] = React.useState("/aeternae/library/manga/seanime/anime-offline-database-minified.json")

    return (
        <div className="space-y-6">
            {/* Status Overview */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold">All-Anime Batch Downloader</h1>
                    <p className="text-muted-foreground mt-2">
                        Download all anime from the offline database in optimized batches
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Badge>
                        {databaseStatus.loaded ? (
                            <>
                                <BiCheckCircle className="w-3 h-3 mr-1" />
                                Database Loaded
                            </>
                        ) : (
                            <>
                                <BiXCircle className="w-3 h-3 mr-1" />
                                Database Not Loaded
                            </>
                        )}
                    </Badge>
                </div>
            </div>

            {/* Database Management */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <BiData className="w-5 h-5" />
                        Anime Database
                    </CardTitle>
                    <CardDescription>
                        Load the anime offline database to begin batch downloading
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    {!databaseStatus.loaded ? (
                        <div className="space-y-4">
                            <div className="space-y-2">
                                <label htmlFor="database-path" className="text-sm font-medium">Database Path</label>
                                <input
                                    id="database-path"
                                    type="text"
                                    value={databasePath}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDatabasePath(e.target.value)}
                                    placeholder="/path/to/anime-offline-database-minified.json"
                                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
                                />
                            </div>
                            <Button
                                onClick={() => onLoadDatabase(databasePath)}
                                disabled={isLoading || !databasePath.trim()}
                                className="w-full"
                            >
                                {isLoading ? (
                                    <>
                                        <LuLoader className="w-4 h-4 mr-2 animate-spin" />
                                        Loading Database...
                                    </>
                                ) : (
                                    <>
                                        <BiData className="w-4 h-4 mr-2" />
                                        Load Database
                                    </>
                                )}
                            </Button>
                        </div>
                    ) : (
                        <div className="space-y-4">
                            <div className="flex items-center justify-between">
                                <span className="text-sm font-medium">Status:</span>
                                <Badge>
                                    <BiCheckCircle className="w-3 h-3 mr-1" />
                                    Loaded Successfully
                                </Badge>
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Total Anime:</span>
                                    <span className="font-mono">{databaseStatus.totalAnime?.toLocaleString()}</span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Database Size:</span>
                                    <span className="font-mono">{databaseStatus.sizeFormatted}</span>
                                </div>
                                <div className="flex justify-between">
                                    <span className="text-muted-foreground">Last Updated:</span>
                                    <span className="font-mono">{databaseStatus.lastModified}</span>
                                </div>
                            </div>
                            {databaseStatus.path && (
                                <div className="p-3 bg-gray-50 dark:bg-gray-900 rounded-lg">
                                    <div className="text-xs text-muted-foreground mb-1">Database Path:</div>
                                    <div className="text-sm font-mono break-all">{databaseStatus.path}</div>
                                </div>
                            )}
                            <Button
                                onClick={() => onLoadDatabase(databasePath)}
                                disabled={isLoading}
                                className="w-full"
                            >
                                {isLoading ? (
                                    <>
                                        <LuLoader className="w-4 h-4 mr-2 animate-spin" />
                                        Reloading...
                                    </>
                                ) : (
                                    <>
                                        <BiRefresh className="w-4 h-4 mr-2" />
                                        Reload Database
                                    </>
                                )}
                            </Button>
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
