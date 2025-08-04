"use client"

import { AL_User } from "@/api/generated/types"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Separator } from "@/components/ui/separator"
import { StaticTabs } from "@/components/ui/tabs"
import { openTab } from "@/lib/helpers/browser"
import Link from "next/link"
import React from "react"
import { BiBook, BiPlay, BiUser } from "react-icons/bi"
import { MdPeople } from "react-icons/md"
import { RiBuilding2Line } from "react-icons/ri"

interface AnilistProfileFavoritesProps {
    user: AL_User
}

export function AnilistProfileFavorites({ user }: AnilistProfileFavoritesProps) {
    const [activeFavTab, setActiveFavTab] = React.useState("anime")

    const favorites = user.favourites

    const favTabs = [
        {
            name: "Anime",
            iconType: BiPlay,
            isCurrent: activeFavTab === "anime",
            onClick: () => setActiveFavTab("anime"),
        },
        {
            name: "Manga",
            iconType: BiBook,
            isCurrent: activeFavTab === "manga",
            onClick: () => setActiveFavTab("manga"),
        },
        {
            name: "Characters",
            iconType: BiUser,
            isCurrent: activeFavTab === "characters",
            onClick: () => setActiveFavTab("characters"),
        },
        {
            name: "Staff",
            iconType: MdPeople,
            isCurrent: activeFavTab === "staff",
            onClick: () => setActiveFavTab("staff"),
        },
        {
            name: "Studios",
            iconType: RiBuilding2Line,
            isCurrent: activeFavTab === "studios",
            onClick: () => setActiveFavTab("studios"),
        },
    ]

    if (!favorites) {
        return (
            <div className="text-center py-8 text-gray-400">
                <p>No favorites data available</p>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <StaticTabs
                items={favTabs}
            />

            <div className="min-h-[400px]">
                {activeFavTab === "anime" && (
                    <FavoriteAnime favorites={favorites.anime} />
                )}
                {activeFavTab === "manga" && (
                    <FavoriteManga favorites={favorites.manga} />
                )}
                {activeFavTab === "characters" && (
                    <FavoriteCharacters favorites={favorites.characters} />
                )}
                {activeFavTab === "staff" && (
                    <FavoriteStaff favorites={favorites.staff} />
                )}
                {activeFavTab === "studios" && (
                    <FavoriteStudios favorites={favorites.studios} />
                )}
            </div>
        </div>
    )
}

function FavoriteAnime({ favorites }: { favorites?: any }) {
    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400">
                <BiPlay className="mx-auto text-4xl mb-2" />
                <p>No favorite anime</p>
            </div>
        )
    }

    return (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
            {favorites.nodes.map((anime: any, index: number) => (
                <MediaEntryCard
                    key={anime.id || index}
                    type="anime"
                    media={anime}
                    showLibraryBadge={false}
                />
            ))}
        </div>
    )
}

function FavoriteManga({ favorites }: { favorites?: any }) {
    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400">
                <BiBook className="mx-auto text-4xl mb-2" />
                <p>No favorite manga</p>
            </div>
        )
    }

    return (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
            {favorites.nodes.map((manga: any, index: number) => (
                <div
                    key={manga.id || index}
                    className="bg-gray-900 rounded-lg overflow-hidden hover:bg-gray-800 transition-colors cursor-pointer"
                    onClick={() => manga.siteUrl && openTab(manga.siteUrl)}
                >
                    {manga.coverImage?.large && (
                        <div className="aspect-[3/4] bg-gray-800">
                            <img
                                src={manga.coverImage.large}
                                alt={manga.title?.romaji || manga.title?.english || "Manga"}
                                className="w-full h-full object-cover"
                            />
                        </div>
                    )}
                    <div className="p-3">
                        <h3 className="text-white font-medium text-sm line-clamp-2">
                            {manga.title?.romaji || manga.title?.english || "Unknown Title"}
                        </h3>
                        {manga.startDate?.year && (
                            <p className="text-gray-400 text-xs mt-1">{manga.startDate.year}</p>
                        )}
                    </div>
                </div>
            ))}
        </div>
    )
}

function FavoriteCharacters({ favorites }: { favorites?: any }) {
    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400">
                <BiUser className="mx-auto text-4xl mb-2" />
                <p>No favorite characters</p>
            </div>
        )
    }

    return (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
            {favorites.nodes.map((character: any, index: number) => (
                <Link
                    key={character.id || index}
                    href={`/character/entry?id=${character.id}`}
                    className="bg-gray-900 rounded-lg overflow-hidden hover:bg-gray-800 transition-colors cursor-pointer block"
                >
                    {character.image?.large && (
                        <div className="aspect-[3/4] bg-gray-800">
                            <img
                                src={character.image.large}
                                alt={character.name?.full || "Character"}
                                className="w-full h-full object-cover"
                            />
                        </div>
                    )}
                    <div className="p-3">
                        <h3 className="text-white font-medium text-sm line-clamp-2">
                            {character.name?.full || character.name?.first || "Unknown Character"}
                        </h3>
                        {character.name?.native && (
                            <p className="text-gray-400 text-xs mt-1">{character.name.native}</p>
                        )}
                    </div>
                </Link>
            ))}
        </div>
    )
}

function FavoriteStaff({ favorites }: { favorites?: any }) {
    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400">
                <MdPeople className="mx-auto text-4xl mb-2" />
                <p>No favorite staff</p>
            </div>
        )
    }

    return (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
            {favorites.nodes.map((staff: any, index: number) => (
                <div
                    key={staff.id || index}
                    className="bg-gray-900 rounded-lg overflow-hidden hover:bg-gray-800 transition-colors cursor-pointer"
                    onClick={() => staff.siteUrl && openTab(staff.siteUrl)}
                >
                    {staff.image?.large && (
                        <div className="aspect-[3/4] bg-gray-800">
                            <img
                                src={staff.image.large}
                                alt={staff.name?.full || "Staff"}
                                className="w-full h-full object-cover"
                            />
                        </div>
                    )}
                    <div className="p-3">
                        <h3 className="text-white font-medium text-sm line-clamp-2">
                            {staff.name?.full || staff.name?.first || "Unknown Staff"}
                        </h3>
                        {staff.name?.native && (
                            <p className="text-gray-400 text-xs mt-1">{staff.name.native}</p>
                        )}
                    </div>
                </div>
            ))}
        </div>
    )
}

function FavoriteStudios({ favorites }: { favorites?: any }) {
    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400">
                <RiBuilding2Line className="mx-auto text-4xl mb-2" />
                <p>No favorite studios</p>
            </div>
        )
    }

    return (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {favorites.nodes.map((studio: any, index: number) => (
                <div
                    key={studio.id || index}
                    className="bg-gray-900 rounded-lg p-4 hover:bg-gray-800 transition-colors cursor-pointer"
                    onClick={() => studio.siteUrl && openTab(studio.siteUrl)}
                >
                    <h3 className="text-white font-medium text-lg">{studio.name}</h3>
                    {studio.isAnimationStudio !== undefined && (
                        <p className="text-gray-400 text-sm mt-1">
                            {studio.isAnimationStudio ? "Animation Studio" : "Studio"}
                        </p>
                    )}
                    {studio.favourites && (
                        <p className="text-blue-400 text-sm mt-2">
                            ❤️ {studio.favourites} favorites
                        </p>
                    )}
                </div>
            ))}
        </div>
    )
}
