"use client"

import { useGetAniListStats } from "@/api/hooks/anilist.hooks"
import { AL_User } from "@/api/generated/types"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Separator } from "@/components/ui/separator"
import { StaticTabs } from "@/components/ui/tabs"
import { cn } from "@/components/ui/core/styling"
import React from "react"
import { BiBook, BiPlay } from "react-icons/bi"
import { Bar, BarChart, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"

interface AnilistProfileStatsProps {
    user: AL_User
}

export function AnilistProfileStats({ user }: AnilistProfileStatsProps) {
    const { data: stats, isLoading } = useGetAniListStats()
    const [activeStatsTab, setActiveStatsTab] = React.useState("anime")

    if (isLoading) {
        return (
            <div className="flex items-center justify-center py-8">
                <LoadingSpinner />
            </div>
        )
    }

    const animeStats = stats?.animeStats
    const mangaStats = stats?.mangaStats

    const statsTabs = [
        {
            name: "Anime",
            iconType: BiPlay,
            isCurrent: activeStatsTab === "anime",
            onClick: () => setActiveStatsTab("anime"),
        },
        {
            name: "Manga",
            iconType: BiBook,
            isCurrent: activeStatsTab === "manga",
            onClick: () => setActiveStatsTab("manga"),
        },
    ]

    const currentStats = activeStatsTab === "anime" ? animeStats : mangaStats

    return (
        <div className="space-y-6">
            <StaticTabs
                items={statsTabs}
            />

            {currentStats && (
                <div className="space-y-8">
                    {/* Overview Stats */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <StatBox
                            title="Total Entries"
                            value={currentStats.count || 0}
                            color="text-blue-400"
                        />
                        <StatBox
                            title={activeStatsTab === "anime" ? "Episodes" : "Chapters"}
                            value={activeStatsTab === "anime" ? (animeStats?.episodesWatched || 0) : (mangaStats?.chaptersRead || 0)}
                            color="text-green-400"
                        />
                        <StatBox
                            title="Mean Score"
                            value={`${currentStats.meanScore || 0}/100`}
                            color="text-yellow-400"
                        />
                        <StatBox
                            title={activeStatsTab === "anime" ? "Days Watched" : "Chapters Read"}
                            value={activeStatsTab === "anime" 
                                ? Math.round((animeStats?.minutesWatched || 0) / 1440)
                                : (mangaStats?.chaptersRead || 0)
                            }
                            color="text-purple-400"
                        />
                    </div>

                    <Separator />

                    {/* Genre Statistics */}
                    {currentStats.genres && currentStats.genres.length > 0 && (
                        <div className="space-y-4">
                            <h3 className="text-xl font-semibold text-white">Genre Distribution</h3>
                            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                                {/* Genre Chart */}
                                <div className="bg-gray-900 rounded-lg p-4">
                                    <h4 className="text-lg font-medium text-white mb-4">Top Genres by Count</h4>
                                    <ResponsiveContainer width="100%" height={300}>
                                        <BarChart data={currentStats.genres.slice(0, 10)}>
                                            <XAxis 
                                                dataKey="genre" 
                                                tick={{ fill: '#9CA3AF', fontSize: 12 }}
                                                angle={-45}
                                                textAnchor="end"
                                                height={80}
                                            />
                                            <YAxis tick={{ fill: '#9CA3AF', fontSize: 12 }} />
                                            <Tooltip 
                                                contentStyle={{ 
                                                    backgroundColor: '#1F2937', 
                                                    border: '1px solid #374151',
                                                    borderRadius: '8px',
                                                    color: '#F3F4F6'
                                                }}
                                            />
                                            <Bar dataKey="amount" fill="#3B82F6" />
                                        </BarChart>
                                    </ResponsiveContainer>
                                </div>

                                {/* Genre List */}
                                <div className="bg-gray-900 rounded-lg p-4">
                                    <h4 className="text-lg font-medium text-white mb-4">Genre Statistics</h4>
                                    <div className="space-y-2 max-h-[300px] overflow-y-auto">
                                        {currentStats.genres.map((genre, index) => (
                                            <div key={index} className="flex justify-between items-center py-2 border-b border-gray-800 last:border-b-0">
                                                <div>
                                                    <div className="text-white font-medium">{genre.genre}</div>
                                                    <div className="text-gray-400 text-sm">{genre.count} entries</div>
                                                </div>
                                                {genre.meanScore && (
                                                    <div className="text-blue-400 font-medium">
                                                        {genre.meanScore}/100
                                                    </div>
                                                )}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Format Statistics */}
                    {activeStatsTab === 'anime' && (currentStats as any).formats && (currentStats as any).formats.length > 0 && (
                        <div className="space-y-4">
                            <h3 className="text-xl font-semibold text-white">Format Distribution</h3>
                            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                                {/* Format Pie Chart */}
                                <div className="bg-gray-900 rounded-lg p-4">
                                    <h4 className="text-lg font-medium text-white mb-4">Format Breakdown</h4>
                                    <ResponsiveContainer width="100%" height={300}>
                                        <PieChart>
                                            <Pie
                                                data={(currentStats as any).formats}
                                                dataKey="amount"
                                                nameKey="format"
                                                cx="50%"
                                                cy="50%"
                                                outerRadius={100}
                                                label={({ format, amount }) => `${format}: ${amount}`}
                                            >
                                                {(currentStats as any).formats.map((entry: any, index: number) => (
                                                    <Cell key={`cell-${index}`} fill={getFormatColor(index)} />
                                                ))}
                                            </Pie>
                                            <Tooltip 
                                                contentStyle={{ 
                                                    backgroundColor: '#1F2937', 
                                                    border: '1px solid #374151',
                                                    borderRadius: '8px',
                                                    color: '#F3F4F6'
                                                }}
                                            />
                                        </PieChart>
                                    </ResponsiveContainer>
                                </div>

                                {/* Format List */}
                                <div className="bg-gray-900 rounded-lg p-4">
                                    <h4 className="text-lg font-medium text-white mb-4">Format Details</h4>
                                    <div className="space-y-2">
                                        {(currentStats as any).formats.map((format: any, index: number) => (
                                            <div key={index} className="flex justify-between items-center py-2">
                                                <div className="flex items-center gap-2">
                                                    <div 
                                                        className="w-3 h-3 rounded-full"
                                                        style={{ backgroundColor: getFormatColor(index) }}
                                                    />
                                                    <span className="text-white">{format.format}</span>
                                                </div>
                                                <span className="text-gray-400">{format.count}</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Status Statistics */}
                    {currentStats.statuses && currentStats.statuses.length > 0 && (
                        <div className="space-y-4">
                            <h3 className="text-xl font-semibold text-white">Status Distribution</h3>
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
                                {currentStats.statuses.map((status, index) => (
                                    <div key={index} className="bg-gray-900 rounded-lg p-4 text-center">
                                        <div className="text-2xl font-bold text-white">{status.count}</div>
                                        <div className="text-gray-400 text-sm capitalize">{status.status?.toLowerCase()}</div>
                                        {status.meanScore && (
                                            <div className="text-blue-400 text-xs mt-1">{status.meanScore}/100</div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

interface StatBoxProps {
    title: string
    value: string | number
    color: string
}

function StatBox({ title, value, color }: StatBoxProps) {
    return (
        <div className="bg-gray-900 rounded-lg p-4 text-center">
            <div className={cn("text-2xl font-bold", color)}>{value}</div>
            <div className="text-gray-400 text-sm">{title}</div>
        </div>
    )
}

function getFormatColor(index: number): string {
    const colors = [
        '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
        '#06B6D4', '#F97316', '#84CC16', '#EC4899', '#6366F1'
    ]
    return colors[index % colors.length]
}
