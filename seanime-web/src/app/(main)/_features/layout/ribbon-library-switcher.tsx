"use client"

import { __library_viewAtom } from "@/app/(main)/(library)/_lib/library-view.atoms"
import { Button } from "@/components/ui/button"
import { useAtom } from "jotai/react"
import { usePathname } from "next/navigation"
import React from "react"
import { BiBook, BiPlay } from "react-icons/bi"

export function RibbonLibrarySwitcher() {
    const [view, setView] = useAtom(__library_viewAtom)
    const pathname = usePathname()
    
    // Only show on library page
    if (pathname !== "/") return null

    return (
        <div className="flex items-center gap-1">
            <Button
                intent={view === 'base' ? 'primary-subtle' : 'gray-subtle'}
                size="sm"
                onClick={() => setView('base')}
                leftIcon={<BiPlay className="w-4 h-4" />}
                className="transition-all duration-200"
            >
                Anime
            </Button>
            <Button
                intent={view === 'manga' ? 'warning-subtle' : 'gray-subtle'}
                size="sm"
                onClick={() => setView('manga')}
                leftIcon={<BiBook className="w-4 h-4" />}
                className="transition-all duration-200"
            >
                Manga
            </Button>
        </div>
    )
}
