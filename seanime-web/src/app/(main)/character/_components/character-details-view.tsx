'use client'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import { BiLinkExternal, BiHeart, BiUser } from 'react-icons/bi'
import Image from 'next/image'
import Link from 'next/link'
import React from 'react'

interface CharacterDetailsViewProps {
    character: any
}

interface MediaEdge {
    id: string
    role: string
    node: {
        id: number
        title: {
            romaji?: string
            english?: string
            native?: string
        }
        type: string
        format?: string
        status?: string
        startDate?: {
            year?: number
        }
        episodes?: number
        chapters?: number
        coverImage?: {
            large?: string
            medium?: string
            color?: string
        }
        averageScore?: number
        popularity?: number
        genres?: string[]
        isAdult?: boolean
    }
}

export function CharacterDetailsView({ character }: CharacterDetailsViewProps) {
    if (!character) {
        return (
            <div className="container mx-auto px-4 py-8">
                <div className="text-center text-muted-foreground">
                    Character not found
                </div>
            </div>
        )
    }

    const formatDate = (dateObj: any) => {
        if (!dateObj) return null
        const { year, month, day } = dateObj
        if (year && month && day) {
            return `${month}/${day}/${year}`
        } else if (year && month) {
            return `${month}/${year}`
        } else if (year) {
            return year.toString()
        }
        return null
    }

    const getPreferredTitle = (title: any) => {
        return title?.english || title?.romaji || title?.native || 'Unknown Title'
    }

    const mainRoleMedia = character.media?.edges?.filter((edge: MediaEdge) => edge.role === 'MAIN') || []
    const supportingRoleMedia = character.media?.edges?.filter((edge: MediaEdge) => edge.role === 'SUPPORTING') || []

    return (
        <div className="min-h-screen bg-gradient-to-br from-gray-950 via-gray-900 to-gray-950">
            <div className="container mx-auto px-4 py-8 max-w-7xl">
                {/* Header Section */}
                <div className="mb-8">
                    <div className="flex flex-col lg:flex-row gap-8">
                        {/* Character Image */}
                        <div className="flex-shrink-0">
                            <Card className="overflow-hidden border-gray-800 bg-gray-900/50 backdrop-blur-sm shadow-2xl">
                                <CardContent className="p-0">
                                    <div className="relative w-80 h-96 mx-auto lg:mx-0">
                                        {character.image?.large ? (
                                            <Image
                                                src={character.image.large}
                                                alt={character.name?.full || 'Character'}
                                                fill
                                                className="object-cover"
                                                sizes="(max-width: 768px) 100vw, 320px"
                                                priority
                                            />
                                        ) : (
                                            <div className="w-full h-full bg-gray-800 flex items-center justify-center">
                                                <BiUser className="w-16 h-16 text-gray-400" />
                                            </div>
                                        )}
                                    </div>
                                </CardContent>
                            </Card>
                        </div>

                        {/* Character Info */}
                        <div className="flex-1 space-y-6">
                            {/* Name and Actions */}
                            <div>
                                <div className="flex items-start justify-between mb-4">
                                    <div>
                                        <h1 className="text-5xl font-bold text-white mb-2 leading-tight">
                                            {character.name?.full || 'Unknown Character'}
                                        </h1>
                                        {character.name?.native && (
                                            <p className="text-2xl text-gray-300 font-medium">
                                                {character.name.native}
                                            </p>
                                        )}
                                    </div>
                                    <div className="flex gap-2">
                                        {character.siteUrl && (
                                            <Button intent="white-outline" size="sm" onClick={() => window.open(character.siteUrl, '_blank')}>
                                                <BiLinkExternal className="w-4 h-4 mr-2" />
                                                AniList
                                            </Button>
                                        )}
                                    </div>
                                </div>

                                {character.name?.alternative && character.name.alternative.length > 0 && (
                                    <div className="mb-4">
                                        <p className="text-sm text-gray-400 mb-2">Alternative Names:</p>
                                        <div className="flex flex-wrap gap-2">
                                            {character.name.alternative.map((alt: string, index: number) => (
                                                <Badge key={index} intent="gray" className="text-sm">
                                                    {alt}
                                                </Badge>
                                            ))}
                                        </div>
                                    </div>
                                )}

                                {/* Stats */}
                                <div className="flex items-center gap-6 text-sm">
                                    {character.favourites && (
                                        <div className="flex items-center gap-1 text-pink-400">
                                            <BiHeart className="w-4 h-4" />
                                            <span>{character.favourites.toLocaleString()} favorites</span>
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Basic Info */}
                            <Card className="border-gray-800 bg-gray-900/50 backdrop-blur-sm">
                                <CardHeader>
                                    <CardTitle className="text-white">Character Information</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                                        {character.gender && (
                                            <div>
                                                <p className="text-sm text-gray-400 mb-1">Gender</p>
                                                <p className="text-white font-medium">{character.gender}</p>
                                            </div>
                                        )}
                                        {character.age && (
                                            <div>
                                                <p className="text-sm text-gray-400 mb-1">Age</p>
                                                <p className="text-white font-medium">{character.age}</p>
                                            </div>
                                        )}
                                        {character.bloodType && (
                                            <div>
                                                <p className="text-sm text-gray-400 mb-1">Blood Type</p>
                                                <p className="text-white font-medium">{character.bloodType}</p>
                                            </div>
                                        )}
                                        {character.dateOfBirth && (
                                            <div>
                                                <p className="text-sm text-gray-400 mb-1">Date of Birth</p>
                                                <p className="text-white font-medium">{formatDate(character.dateOfBirth) || 'Unknown'}</p>
                                            </div>
                                        )}
                                        {character.favourites && (
                                            <div>
                                                <p className="text-sm text-gray-400 mb-1">Favourites</p>
                                                <p className="text-white">{character.favourites.toLocaleString()}</p>
                                            </div>
                                        )}
                                    </div>
                                </CardContent>
                            </Card>
                        </div>
                    </div>
                </div>

                {/* Description */}
                {character.description && (
                    <Card className="mb-8 border-gray-800 bg-gray-900/50 backdrop-blur-sm">
                        <CardContent className="p-6">
                            <h2 className="text-2xl font-semibold text-white mb-4">Description</h2>
                            <div 
                                className="text-gray-300 leading-relaxed prose prose-invert max-w-none"
                                dangerouslySetInnerHTML={{ 
                                    __html: character.description.replace(/\n/g, '<br />') 
                                }}
                            />
                        </CardContent>
                    </Card>
                )}

                {/* Media Appearances */}
                {character.media && character.media.edges && character.media.edges.length > 0 && (
                    <Card className="border-gray-800 bg-gray-900/50 backdrop-blur-sm">
                        <CardContent className="p-6">
                            <h2 className="text-2xl font-semibold text-white mb-6">Media Appearances</h2>
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                                {character.media.edges.map((edge: any, index: number) => {
                                    const media = edge.node
                                    if (!media) return null

                                    return (
                                        <Link 
                                            key={media.id || index} 
                                            href={`/${media.type?.toLowerCase() || 'anime'}/entry?id=${media.id}`}
                                            className="group"
                                        >
                                            <Card className="overflow-hidden border-gray-700 bg-gray-800/50 backdrop-blur-sm transition-all duration-200 group-hover:border-gray-600 group-hover:bg-gray-700/50 group-hover:scale-105">
                                                <CardContent className="p-0">
                                                    <div className="relative aspect-[3/4] w-full">
                                                        {media.coverImage?.large ? (
                                                            <Image
                                                                src={media.coverImage.large}
                                                                alt={media.title?.romaji || media.title?.english || 'Media'}
                                                                fill
                                                                className="object-cover"
                                                                sizes="(max-width: 768px) 50vw, (max-width: 1200px) 33vw, 25vw"
                                                            />
                                                        ) : (
                                                            <div className="w-full h-full bg-gray-700 flex items-center justify-center">
                                                                <span className="text-gray-400 text-sm">No Image</span>
                                                            </div>
                                                        )}
                                                        {/* Role Badge */}
                                                        {edge.role && (
                                                            <div className="absolute top-2 right-2">
                                                                <Badge 
                                                                    intent={edge.role === 'MAIN' ? 'primary' : 'gray'}
                                                                    className={cn(
                                                                        "text-xs",
                                                                        edge.role === 'MAIN' && "bg-blue-600 hover:bg-blue-700"
                                                                    )}
                                                                >
                                                                    {edge.role}
                                                                </Badge>
                                                            </div>
                                                        )}
                                                    </div>
                                                    <div className="p-3">
                                                        <h3 className="font-medium text-white text-sm line-clamp-2 mb-1">
                                                            {media.title?.romaji || media.title?.english || 'Unknown Title'}
                                                        </h3>
                                                        <div className="flex items-center justify-between text-xs text-gray-400">
                                                            <span>{media.type}</span>
                                                            {media.startDate?.year && (
                                                                <span>{media.startDate.year}</span>
                                                            )}
                                                        </div>
                                                    </div>
                                                </CardContent>
                                            </Card>
                                        </Link>
                                    )
                                })}
                            </div>
                        </CardContent>
                    </Card>
                )}
            </div>
        </div>
    )
}
