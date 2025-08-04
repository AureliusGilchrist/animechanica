"use client"

import { AL_User } from "@/api/generated/types"
import { Avatar } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { Modal } from "@/components/ui/modal"
import { Textarea } from "@/components/ui/textarea"
import { openTab } from "@/lib/helpers/browser"
import { formatDistanceToNow } from "date-fns"
import React from "react"
import { BiCalendar, BiEdit, BiLink, BiSave, BiX } from "react-icons/bi"
import { SiAnilist } from "react-icons/si"
import { toast } from "sonner"
import { useUpdateAniListProfile } from "@/api/hooks/anilist_profile_sync.hooks"

interface AnilistProfileBannerEditableProps {
    user: AL_User
    onBioUpdate?: (newBio: string) => void
}

export function AnilistProfileBannerEditable({ user, onBioUpdate }: AnilistProfileBannerEditableProps) {
    const [isEditingBio, setIsEditingBio] = React.useState(false)
    const [bioText, setBioText] = React.useState(user.about || "")
    const [isSaving, setIsSaving] = React.useState(false)

    const joinDate = user.createdAt ? new Date(user.createdAt * 1000) : null
    const bannerImage = user.bannerImage

    const updateProfileMutation = useUpdateAniListProfile()

    const handleSaveBio = async () => {
        setIsSaving(true)
        try {
            // Update profile on AniList
            await updateProfileMutation.mutateAsync({
                about: bioText
            })
            
            onBioUpdate?.(bioText)
            setIsEditingBio(false)
        } catch (error) {
            // Error handling is done in the mutation hook
        } finally {
            setIsSaving(false)
        }
    }

    const handleCancelEdit = () => {
        setBioText(user.about || "")
        setIsEditingBio(false)
    }

    return (
        <>
            <div className="relative overflow-hidden rounded-lg border bg-gray-950">
                {/* Banner Image */}
                {bannerImage && (
                    <div 
                        className="h-48 bg-cover bg-center bg-no-repeat"
                        style={{ backgroundImage: `url(${bannerImage})` }}
                    >
                        <div className="absolute inset-0 bg-gradient-to-t from-gray-950 via-gray-950/60 to-transparent" />
                    </div>
                )}
                
                {/* Profile Content */}
                <div className={cn(
                    "relative p-6",
                    bannerImage ? "-mt-20" : ""
                )}>
                    <div className="flex flex-col sm:flex-row gap-6 items-start">
                        {/* Avatar */}
                        <div className="relative z-10">
                            <Avatar 
                                src={user.avatar?.large || user.avatar?.medium}
                                size="xl"
                                className="ring-4 ring-gray-950 bg-gray-800"
                            />
                        </div>

                        {/* User Info */}
                        <div className="flex-1 space-y-3">
                            <div className="flex flex-col sm:flex-row sm:items-center gap-3">
                                <h1 className="text-3xl font-bold text-white">
                                    {user.name}
                                </h1>
                                
                                <div className="flex items-center gap-2">
                                    {user.donatorTier && user.donatorTier > 0 && (
                                        <Badge 
                                            intent="primary-solid"
                                            className="bg-pink-500 text-white"
                                        >
                                            Donator
                                        </Badge>
                                    )}
                                    
                                    {user.moderatorRoles && user.moderatorRoles.length > 0 && (
                                        <Badge 
                                            intent="warning-solid"
                                            className="bg-orange-500 text-white"
                                        >
                                            Moderator
                                        </Badge>
                                    )}
                                </div>
                            </div>

                            {/* Bio Section */}
                            <div className="space-y-2">
                                <div className="flex items-center gap-2">
                                    <span className="text-sm font-medium text-gray-400">Biography</span>
                                    <Button
                                        size="sm"
                                        intent="gray-outline"
                                        onClick={() => setIsEditingBio(true)}
                                        className="h-6 px-2 text-xs"
                                    >
                                        <BiEdit className="w-3 h-3" />
                                        Edit
                                    </Button>
                                </div>
                                
                                {user.about ? (
                                    <div 
                                        className="text-gray-300 prose prose-sm max-w-none prose-invert"
                                        dangerouslySetInnerHTML={{ __html: user.about }}
                                    />
                                ) : (
                                    <p className="text-gray-500 italic">No biography set</p>
                                )}
                            </div>

                            {/* Meta Info */}
                            <div className="flex flex-wrap items-center gap-4 text-sm text-gray-400">
                                {joinDate && (
                                    <div className="flex items-center gap-1">
                                        <BiCalendar className="text-base" />
                                        <span>
                                            Joined {formatDistanceToNow(joinDate, { addSuffix: true })}
                                        </span>
                                    </div>
                                )}
                                
                                {user.siteUrl && (
                                    <button
                                        onClick={() => openTab(user.siteUrl!)}
                                        className="flex items-center gap-1 hover:text-blue-400 transition-colors"
                                    >
                                        <SiAnilist className="text-base" />
                                        <span>View on AniList</span>
                                        <BiLink className="text-xs" />
                                    </button>
                                )}
                            </div>

                            {/* Previous Names */}
                            {user.previousNames && user.previousNames.length > 0 && (
                                <div className="text-sm text-gray-400">
                                    <span className="font-medium">Previous names: </span>
                                    {user.previousNames.map((prev, index) => (
                                        <span key={index}>
                                            {prev.name}
                                            {index < user.previousNames!.length - 1 && ", "}
                                        </span>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {/* Bio Edit Modal */}
            <Modal
                open={isEditingBio}
                onOpenChange={setIsEditingBio}
                title="Edit Biography"
                contentClass="max-w-2xl"
            >
                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-300 mb-2">
                            Biography
                        </label>
                        <Textarea
                            value={bioText}
                            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setBioText(e.target.value)}
                            placeholder="Tell us about yourself..."
                            rows={6}
                            className="w-full"
                        />
                        <p className="text-xs text-gray-500 mt-1">
                            You can use HTML tags for formatting
                        </p>
                    </div>

                    <div className="flex justify-end gap-2">
                        <Button
                            intent="gray-outline"
                            onClick={handleCancelEdit}
                            disabled={isSaving}
                        >
                            <BiX className="w-4 h-4 mr-1" />
                            Cancel
                        </Button>
                        <Button
                            onClick={handleSaveBio}
                            loading={isSaving}
                        >
                            <BiSave className="w-4 h-4 mr-1" />
                            Save Biography
                        </Button>
                    </div>
                </div>
            </Modal>
        </>
    )
}
