"use client"

import { AL_User } from "@/api/generated/types"
import { Button } from "@/components/ui/button"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { Separator } from "@/components/ui/separator"
import { StaticTabs } from "@/components/ui/tabs"
import { TextInput } from "@/components/ui/text-input"
import { openTab } from "@/lib/helpers/browser"
import React from "react"
import { BiBook, BiEdit, BiPlay, BiPlus, BiSearch, BiTrash, BiUser, BiX } from "react-icons/bi"
import { MdDragIndicator, MdPeople } from "react-icons/md"
import { RiBuilding2Line } from "react-icons/ri"
import { toast } from "sonner"
import { useToggleFavoriteAniList } from "@/api/hooks/anilist_profile_sync.hooks"

interface AnilistProfileFavoritesEditableProps {
    user: AL_User
    onFavoritesUpdate?: (type: string, favorites: any[]) => void
}

export function AnilistProfileFavoritesEditable({ user, onFavoritesUpdate }: AnilistProfileFavoritesEditableProps) {
    const [activeFavTab, setActiveFavTab] = React.useState("anime")
    const [isEditing, setIsEditing] = React.useState(false)
    const [editingType, setEditingType] = React.useState<string>("")
    const [searchQuery, setSearchQuery] = React.useState("")
    const [searchResults, setSearchResults] = React.useState<any[]>([])
    const [isSearching, setIsSearching] = React.useState(false)
    const [localFavorites, setLocalFavorites] = React.useState<any[]>([])

    const favorites = user.favourites

    const favTabs = [
        {
            name: "Anime",
            href: null,
            iconType: BiPlay,
            isCurrent: activeFavTab === "anime",
            onClick: () => setActiveFavTab("anime"),
        },
        {
            name: "Manga",
            href: null,
            iconType: BiBook,
            isCurrent: activeFavTab === "manga",
            onClick: () => setActiveFavTab("manga"),
        },
        {
            name: "Characters",
            href: null,
            iconType: BiUser,
            isCurrent: activeFavTab === "characters",
            onClick: () => setActiveFavTab("characters"),
        },
        {
            name: "Staff",
            href: null,
            iconType: MdPeople,
            isCurrent: activeFavTab === "staff",
            onClick: () => setActiveFavTab("staff"),
        },
        {
            name: "Studios",
            href: null,
            iconType: RiBuilding2Line,
            isCurrent: activeFavTab === "studios",
            onClick: () => setActiveFavTab("studios"),
        },
    ]

    const handleStartEdit = (type: string) => {
        setEditingType(type)
        const currentFavorites = favorites?.[type as keyof typeof favorites]?.nodes || []
        setLocalFavorites([...currentFavorites])
        setIsEditing(true)
        setSearchQuery("")
        setSearchResults([])
    }

    const handleSearch = async () => {
        if (!searchQuery.trim()) return
        
        setIsSearching(true)
        try {
            // Simulate search API call
            await new Promise(resolve => setTimeout(resolve, 1000))
            
            // Mock search results based on type
            const mockResults = Array.from({ length: 5 }, (_, i) => ({
                id: `search_${i}`,
                title: { romaji: `${searchQuery} Result ${i + 1}` },
                name: { full: `${searchQuery} Character ${i + 1}` },
                coverImage: { large: `https://via.placeholder.com/300x400?text=${searchQuery}+${i + 1}` },
                image: { large: `https://via.placeholder.com/300x400?text=${searchQuery}+${i + 1}` },
            }))
            
            setSearchResults(mockResults)
        } catch (error) {
            toast.error("Failed to search")
        } finally {
            setIsSearching(false)
        }
    }

    const handleAddFavorite = (item: any) => {
        if (localFavorites.find(fav => fav.id === item.id)) {
            toast.error("Already in favorites")
            return
        }
        
        setLocalFavorites(prev => [...prev, item])
        toast.success("Added to favorites")
    }

    const handleRemoveFavorite = (id: string) => {
        setLocalFavorites(prev => prev.filter(fav => fav.id !== id))
        toast.success("Removed from favorites")
    }

    const handleReorderFavorites = (fromIndex: number, toIndex: number) => {
        const newFavorites = [...localFavorites]
        const [removed] = newFavorites.splice(fromIndex, 1)
        newFavorites.splice(toIndex, 0, removed)
        setLocalFavorites(newFavorites)
    }

    const handleSaveFavorites = async () => {
        try {
            // Simulate API call
            await new Promise(resolve => setTimeout(resolve, 1000))
            
            onFavoritesUpdate?.(editingType, localFavorites)
            setIsEditing(false)
            toast.success("Favorites updated successfully!")
        } catch (error) {
            toast.error("Failed to update favorites")
        }
    }

    if (!favorites) {
        return (
            <div className="text-center py-8 text-gray-400">
                <p>No favorites data available</p>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <StaticTabs items={favTabs} />

            <div className="min-h-[400px]">
                {activeFavTab === "anime" && (
                    <FavoriteSection
                        type="anime"
                        favorites={favorites.anime}
                        onEdit={() => handleStartEdit("anime")}
                    />
                )}
                {activeFavTab === "manga" && (
                    <FavoriteSection
                        type="manga"
                        favorites={favorites.manga}
                        onEdit={() => handleStartEdit("manga")}
                    />
                )}
                {activeFavTab === "characters" && (
                    <FavoriteSection
                        type="characters"
                        favorites={favorites.characters}
                        onEdit={() => handleStartEdit("characters")}
                    />
                )}
                {activeFavTab === "staff" && (
                    <FavoriteSection
                        type="staff"
                        favorites={favorites.staff}
                        onEdit={() => handleStartEdit("staff")}
                    />
                )}
                {activeFavTab === "studios" && (
                    <FavoriteSection
                        type="studios"
                        favorites={favorites.studios}
                        onEdit={() => handleStartEdit("studios")}
                    />
                )}
            </div>

            {/* Edit Favorites Modal */}
            <Modal
                open={isEditing}
                onOpenChange={setIsEditing}
                title={`Edit ${editingType} Favorites`}
                contentClass="max-w-4xl"
            >
                <div className="space-y-6">
                    {/* Search Section */}
                    <div className="space-y-4">
                        <h3 className="text-lg font-semibold text-white">Add New Favorites</h3>
                        <div className="flex gap-2">
                            <TextInput
                                value={searchQuery}
                                onChange={(e) => setSearchQuery(e.target.value)}
                                placeholder={`Search for ${editingType}...`}
                                className="flex-1"
                                onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
                            />
                            <Button onClick={handleSearch} loading={isSearching}>
                                <BiSearch className="w-4 h-4" />
                            </Button>
                        </div>

                        {/* Search Results */}
                        {searchResults.length > 0 && (
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 max-h-60 overflow-y-auto">
                                {searchResults.map((item, index) => (
                                    <SearchResultCard
                                        key={index}
                                        item={item}
                                        type={editingType}
                                        onAdd={() => handleAddFavorite(item)}
                                    />
                                ))}
                            </div>
                        )}
                    </div>

                    <Separator />

                    {/* Current Favorites */}
                    <div className="space-y-4">
                        <h3 className="text-lg font-semibold text-white">
                            Current Favorites ({localFavorites.length})
                        </h3>
                        
                        {localFavorites.length === 0 ? (
                            <p className="text-gray-400 text-center py-8">No favorites yet</p>
                        ) : (
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 max-h-80 overflow-y-auto">
                                {localFavorites.map((item, index) => (
                                    <EditableFavoriteCard
                                        key={item.id}
                                        item={item}
                                        type={editingType}
                                        index={index}
                                        onRemove={() => handleRemoveFavorite(item.id)}
                                        onReorder={handleReorderFavorites}
                                    />
                                ))}
                            </div>
                        )}
                    </div>

                    {/* Actions */}
                    <div className="flex justify-end gap-2">
                        <Button
                            intent="gray-outline"
                            onClick={() => setIsEditing(false)}
                        >
                            <BiX className="w-4 h-4 mr-1" />
                            Cancel
                        </Button>
                        <Button onClick={handleSaveFavorites}>
                            Save Changes
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    )
}

function FavoriteSection({ type, favorites, onEdit }: { type: string, favorites?: any, onEdit: () => void }) {
    const Icon = type === "anime" ? BiPlay : type === "manga" ? BiBook : type === "characters" ? BiUser : type === "staff" ? MdPeople : RiBuilding2Line

    if (!favorites?.nodes || favorites.nodes.length === 0) {
        return (
            <div className="text-center py-8 text-gray-400 space-y-4">
                <Icon className="mx-auto text-4xl mb-2" />
                <p>No favorite {type}</p>
                <Button onClick={onEdit} size="sm">
                    <BiPlus className="w-4 h-4 mr-1" />
                    Add Favorites
                </Button>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            <div className="flex justify-between items-center">
                <h3 className="text-lg font-semibold text-white capitalize">
                    Favorite {type} ({favorites.nodes.length})
                </h3>
                <Button onClick={onEdit} size="sm" intent="gray-outline">
                    <BiEdit className="w-4 h-4 mr-1" />
                    Edit
                </Button>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                {favorites.nodes.map((item: any, index: number) => (
                    <FavoriteCard key={item.id || index} item={item} type={type} />
                ))}
            </div>
        </div>
    )
}

function FavoriteCard({ item, type }: { item: any, type: string }) {
    const getTitle = () => {
        if (type === "studios") return item.name
        if (type === "characters" || type === "staff") return item.name?.full || item.name?.first
        return item.title?.romaji || item.title?.english || "Unknown"
    }

    const getImage = () => {
        if (type === "studios") return null
        if (type === "characters" || type === "staff") return item.image?.large
        return item.coverImage?.large
    }

    return (
        <div
            className="bg-gray-900 rounded-lg overflow-hidden hover:bg-gray-800 transition-colors cursor-pointer"
            onClick={() => item.siteUrl && openTab(item.siteUrl)}
        >
            {getImage() && (
                <div className="aspect-[3/4] bg-gray-800">
                    <img
                        src={getImage()}
                        alt={getTitle()}
                        className="w-full h-full object-cover"
                    />
                </div>
            )}
            <div className="p-3">
                <h3 className="text-white font-medium text-sm line-clamp-2">
                    {getTitle()}
                </h3>
            </div>
        </div>
    )
}

function SearchResultCard({ item, type, onAdd }: { item: any, type: string, onAdd: () => void }) {
    const getTitle = () => {
        if (type === "studios") return item.name
        if (type === "characters" || type === "staff") return item.name?.full || item.name?.first
        return item.title?.romaji || item.title?.english || "Unknown"
    }

    return (
        <div className="bg-gray-800 rounded-lg overflow-hidden">
            {item.coverImage?.large || item.image?.large ? (
                <div className="aspect-[3/4] bg-gray-700">
                    <img
                        src={item.coverImage?.large || item.image?.large}
                        alt={getTitle()}
                        className="w-full h-full object-cover"
                    />
                </div>
            ) : null}
            <div className="p-2">
                <h4 className="text-white text-xs font-medium line-clamp-2 mb-2">
                    {getTitle()}
                </h4>
                <Button size="xs" onClick={onAdd} className="w-full">
                    <BiPlus className="w-3 h-3 mr-1" />
                    Add
                </Button>
            </div>
        </div>
    )
}

function EditableFavoriteCard({ item, type, index, onRemove, onReorder }: { 
    item: any, 
    type: string, 
    index: number,
    onRemove: () => void,
    onReorder: (fromIndex: number, toIndex: number) => void
}) {
    const getTitle = () => {
        if (type === "studios") return item.name
        if (type === "characters" || type === "staff") return item.name?.full || item.name?.first
        return item.title?.romaji || item.title?.english || "Unknown"
    }

    return (
        <div className="bg-gray-800 rounded-lg overflow-hidden relative group">
            <div className="absolute top-1 left-1 z-10 opacity-0 group-hover:opacity-100 transition-opacity">
                <MdDragIndicator className="text-gray-400 cursor-move" />
            </div>
            <div className="absolute top-1 right-1 z-10 opacity-0 group-hover:opacity-100 transition-opacity">
                <Button size="xs" intent="alert-outline" onClick={onRemove} className="bg-red-600 hover:bg-red-700">
                    <BiTrash className="w-3 h-3" />
                </Button>
            </div>
            
            {item.coverImage?.large || item.image?.large ? (
                <div className="aspect-[3/4] bg-gray-700">
                    <img
                        src={item.coverImage?.large || item.image?.large}
                        alt={getTitle()}
                        className="w-full h-full object-cover"
                    />
                </div>
            ) : null}
            <div className="p-2">
                <h4 className="text-white text-xs font-medium line-clamp-2">
                    {getTitle()}
                </h4>
            </div>
        </div>
    )
}
