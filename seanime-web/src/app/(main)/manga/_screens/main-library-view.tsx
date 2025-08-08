import React, { useState } from "react"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { DownloadedMangaLibraryView } from "./downloaded-manga-library-view"
import { MangaLibraryView } from "./manga-library-view"
import { Manga_Collection } from "@/api/generated/types"

type MainLibraryViewProps = {
    // Props from existing manga library
    collection?: Manga_Collection
    filteredCollection?: Manga_Collection | undefined
    genres?: string[]
    storedProviders?: Record<string, string>
    hasManga?: boolean
}

export function MainLibraryView(props: MainLibraryViewProps) {
    const {
        collection,
        filteredCollection,
        genres,
        storedProviders,
        hasManga,
    } = props

    return (
        <PageWrapper className="space-y-6">
            {/* Header with toggle */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold">Library</h1>
                    <p className="text-muted-foreground mt-1">
                        Manage your anime and manga collection
                    </p>
                </div>
            </div>

            {/* Tabs for Anime/Manga toggle */}
            <Tabs defaultValue="manga" className="w-full">
                <TabsList className="grid w-full max-w-md grid-cols-2">
                    <TabsTrigger value="anime" disabled>
                        Anime
                        <span className="ml-2 text-xs text-muted-foreground">(Coming Soon)</span>
                    </TabsTrigger>
                    <TabsTrigger value="manga">
                        Manga
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="anime" className="space-y-6">
                    <div className="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
                        <div className="text-center">
                            <h3 className="text-lg font-medium text-muted-foreground">Anime Library Coming Soon</h3>
                            <p className="text-sm text-muted-foreground mt-2">
                                Downloaded anime series will be displayed here in a future update
                            </p>
                        </div>
                    </div>
                </TabsContent>

                <TabsContent value="manga" className="space-y-6">
                    {/* Sub-tabs for Online vs Downloaded manga */}
                    <Tabs defaultValue="downloaded" className="w-full">
                        <TabsList className="grid w-full max-w-md grid-cols-2">
                            <TabsTrigger value="online">
                                Online Collection
                            </TabsTrigger>
                            <TabsTrigger value="downloaded">
                                Downloaded Series
                            </TabsTrigger>
                        </TabsList>

                        <TabsContent value="online" className="space-y-6">
                            {collection && (
                                <MangaLibraryView
                                    collection={collection}
                                    filteredCollection={filteredCollection}
                                    genres={genres || []}
                                    storedProviders={storedProviders || {}}
                                    hasManga={hasManga || false}
                                />
                            )}
                        </TabsContent>

                        <TabsContent value="downloaded" className="space-y-6">
                            <DownloadedMangaLibraryView />
                        </TabsContent>
                    </Tabs>
                </TabsContent>
            </Tabs>
        </PageWrapper>
    )
}
