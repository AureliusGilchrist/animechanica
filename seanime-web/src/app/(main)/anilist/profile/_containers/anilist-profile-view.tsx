"use client"

import { useCurrentUser } from "@/app/(main)/_hooks/use-server-status"
import { AnilistProfileBannerEditable } from "@/app/(main)/anilist/profile/_components/anilist-profile-banner-editable"
import { AnilistProfileFavoritesEditable } from "@/app/(main)/anilist/profile/_components/anilist-profile-favorites-editable"
import { AnilistProfileOverview } from "@/app/(main)/anilist/profile/_components/anilist-profile-overview"
import { AnilistProfileStats } from "@/app/(main)/anilist/profile/_components/anilist-profile-stats"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { StaticTabs } from "@/components/ui/tabs"
import React from "react"
import { BiStats, BiUser } from "react-icons/bi"
import { MdFavorite } from "react-icons/md"

export function AnilistProfileView() {
    const user = useCurrentUser()
    const [activeTab, setActiveTab] = React.useState("overview")
    const [userProfile, setUserProfile] = React.useState(user)

    // Update local profile state when user data changes
    React.useEffect(() => {
        if (user) {
            setUserProfile(user)
        }
    }, [user])

    const handleBioUpdate = (newBio: string) => {
        if (userProfile) {
            setUserProfile({
                ...userProfile,
                about: newBio,
            } as any)
        }
    }

    const handleFavoritesUpdate = (type: string, favorites: any[]) => {
        if (userProfile && (userProfile as any)?.favourites) {
            setUserProfile({
                ...userProfile,
                favourites: {
                    ...(userProfile as any).favourites,
                    [type]: {
                        nodes: favorites,
                    },
                },
            } as any)
        }
    }

    if (!user || !userProfile) {
        return (
            <div className="flex items-center justify-center min-h-[50vh]">
                <LoadingSpinner />
            </div>
        )
    }

    const tabItems = [
        {
            name: "Overview",
            href: null,
            iconType: BiUser,
            isCurrent: activeTab === "overview",
            onClick: () => setActiveTab("overview"),
        },
        {
            name: "Statistics",
            href: null,
            iconType: BiStats,
            isCurrent: activeTab === "statistics",
            onClick: () => setActiveTab("statistics"),
        },
        {
            name: "Favorites",
            href: null,
            iconType: MdFavorite,
            isCurrent: activeTab === "favorites",
            onClick: () => setActiveTab("favorites"),
        },
    ]

    return (
        <AppLayoutStack>
            <AnilistProfileBannerEditable 
                user={userProfile as any} 
                onBioUpdate={handleBioUpdate}
            />
            
            <div className="space-y-6">
                <StaticTabs
                    items={tabItems}
                />

                {activeTab === "overview" && <AnilistProfileOverview user={userProfile as any} />}
                {activeTab === "statistics" && <AnilistProfileStats user={userProfile as any} />}
                {activeTab === "favorites" && <AnilistProfileFavoritesEditable 
                    user={userProfile as any} 
                    onFavoritesUpdate={handleFavoritesUpdate}
                />}
            </div>
        </AppLayoutStack>
    )
}
