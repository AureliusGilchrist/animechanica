"use client"

import { useGetAniListStats } from "@/api/hooks/anilist.hooks"
import { AL_User } from "@/api/generated/types"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Separator } from "@/components/ui/separator"
import { cn } from "@/components/ui/core/styling"
import React from "react"
import { BiBook, BiPlay, BiTime } from "react-icons/bi"
import { MdOutlineScore } from "react-icons/md"

interface AnilistProfileOverviewProps {
    user: AL_User
}

export function AnilistProfileOverview({ user }: AnilistProfileOverviewProps) {
    const { data: stats, isLoading } = useGetAniListStats()

    if (isLoading) {
        return (
            <div className="flex items-center justify-center py-8">
                <LoadingSpinner />
            </div>
        )
    }

    const animeStats = stats?.animeStats
    const mangaStats = stats?.mangaStats

    return (
        <div className="space-y-6">
            {/* Quick Stats Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <StatCard
                    icon={BiPlay}
                    title="Anime Watched"
                    value={animeStats?.count || 0}
                    subtitle={`${Math.round((animeStats?.minutesWatched || 0) / 60)} hours`}
                    color="text-blue-400"
                />
                <StatCard
                    icon={BiBook}
                    title="Manga Read"
                    value={mangaStats?.count || 0}
                    subtitle={`${mangaStats?.chaptersRead || 0} chapters`}
                    color="text-green-400"
                />
                <StatCard
                    icon={MdOutlineScore}
                    title="Mean Score"
                    value={animeStats?.meanScore ? `${animeStats.meanScore}/100` : "N/A"}
                    subtitle="Anime rating"
                    color="text-yellow-400"
                />
                <StatCard
                    icon={BiTime}
                    title="Days Watched"
                    value={Math.round((animeStats?.minutesWatched || 0) / 1440)}
                    subtitle="Total anime time"
                    color="text-purple-400"
                />
            </div>

            <Separator />

            {/* Activity Overview */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Anime Overview */}
                <div className="space-y-4">
                    <h3 className="text-xl font-semibold text-white flex items-center gap-2">
                        <BiPlay className="text-blue-400" />
                        Anime Overview
                    </h3>
                    
                    <div className="bg-gray-900 rounded-lg p-4 space-y-3">
                        <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                                <span className="text-gray-400">Total Entries:</span>
                                <div className="text-white font-medium">{animeStats?.count || 0}</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Rewatched:</span>
                                <div className="text-white font-medium">{animeStats?.episodesWatched || 0}</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Mean Score:</span>
                                <div className="text-white font-medium">{animeStats?.meanScore || 0}/100</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Episodes Watched:</span>
                                <div className="text-white font-medium">{animeStats?.episodesWatched || 0}</div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Manga Overview */}
                <div className="space-y-4">
                    <h3 className="text-xl font-semibold text-white flex items-center gap-2">
                        <BiBook className="text-green-400" />
                        Manga Overview
                    </h3>
                    
                    <div className="bg-gray-900 rounded-lg p-4 space-y-3">
                        <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                                <span className="text-gray-400">Total Entries:</span>
                                <div className="text-white font-medium">{mangaStats?.count || 0}</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Chapters Read:</span>
                                <div className="text-white font-medium">{mangaStats?.chaptersRead || 0}</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Mean Score:</span>
                                <div className="text-white font-medium">{mangaStats?.meanScore || 0}/100</div>
                            </div>
                            <div>
                                <span className="text-gray-400">Chapters Read:</span>
                                <div className="text-white font-medium">{mangaStats?.chaptersRead || 0}</div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Genre Distribution Preview */}
            {animeStats?.genres && animeStats.genres.length > 0 && (
                <div className="space-y-4">
                    <h3 className="text-xl font-semibold text-white">Top Genres</h3>
                    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                        {animeStats.genres.slice(0, 8).map((genre, index) => (
                            <div key={index} className="bg-gray-900 rounded-lg p-3 text-center">
                                <div className="text-white font-medium text-sm">{genre.genre}</div>
                                <div className="text-gray-400 text-xs">{genre.count} entries</div>
                                {genre.meanScore && (
                                    <div className="text-blue-400 text-xs">{genre.meanScore}/100</div>
                                )}
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}

interface StatCardProps {
    icon: React.ComponentType<any>
    title: string
    value: string | number
    subtitle: string
    color: string
}

function StatCard({ icon: Icon, title, value, subtitle, color }: StatCardProps) {
    return (
        <div className="bg-gray-900 rounded-lg p-4 space-y-2">
            <div className="flex items-center gap-2">
                <Icon className={cn("text-xl", color)} />
                <span className="text-gray-400 text-sm">{title}</span>
            </div>
            <div className="text-2xl font-bold text-white">{value}</div>
            <div className="text-xs text-gray-500">{subtitle}</div>
        </div>
    )
}
