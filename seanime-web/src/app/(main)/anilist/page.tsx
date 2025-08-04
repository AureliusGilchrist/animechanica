"use client"

import { CustomLibraryBanner } from "@/app/(main)/(library)/_containers/custom-library-banner"
import { AnilistCollectionLists } from "@/app/(main)/anilist/_containers/anilist-collection-lists"
import { AnilistProfileView } from "@/app/(main)/anilist/profile/_containers/anilist-profile-view"
import { useCurrentUser } from "@/app/(main)/_hooks/use-server-status"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { StaticTabs } from "@/components/ui/tabs"
import React from "react"
import { BiUser } from "react-icons/bi"
import { IoLibrary } from "react-icons/io5"

export const dynamic = "force-static"

export default function AnilistPage() {
    const user = useCurrentUser()
    const [activeTab, setActiveTab] = React.useState("lists")

    const tabItems = [
        {
            name: "My Lists",
            href: null,
            iconType: IoLibrary,
            isCurrent: activeTab === "lists",
            onClick: () => setActiveTab("lists"),
        },
        {
            name: "Profile",
            href: null,
            iconType: BiUser,
            isCurrent: activeTab === "profile",
            onClick: () => setActiveTab("profile"),
        },
    ]

    return (
        <>
            <CustomLibraryBanner discrete />
            <PageWrapper
                className="p-4 sm:p-8 pt-4 relative"
                data-anilist-page
                {...{
                    initial: { opacity: 0, y: 10 },
                    animate: { opacity: 1, y: 0 },
                    exit: { opacity: 0, y: 10 },
                    transition: {
                        type: "spring",
                        damping: 20,
                        stiffness: 100,
                    },
                }}
            >
                <div className="space-y-6">
                    <StaticTabs
                        items={tabItems}
                    />

                    {activeTab === "lists" && <AnilistCollectionLists />}
                    {activeTab === "profile" && <AnilistProfileView />}
                </div>
            </PageWrapper>
        </>
    )
}
