import React from "react"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { DownloadedMangaLibraryView } from "./downloaded-manga-library-view"
import { Manga_Collection } from "@/api/generated/types"
import { TextInput } from "@/components/ui/text-input"
import { IconButton } from "@/components/ui/button"
import { MediaGenreSelector } from "@/app/(main)/_features/media/_components/media-genre-selector"
import { useDebounce } from "@/hooks/use-debounce"
import { FiSearch, FiX } from "react-icons/fi"

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

    // Local search state for downloaded manga view
    const [search, setSearch] = React.useState("")
    const debouncedSearch = useDebounce(search, 250)
    const [selectedGenres, setSelectedGenres] = React.useState<string[]>([])

    // Build a quick lookup of mediaId -> genres from the provided collection data
    const mediaGenresById = React.useMemo(() => {
        const map: Record<number, string[]> = {}
        try {
            const lists = collection?.lists || []
            for (const list of lists) {
                const entries = list?.entries || []
                for (const e of entries) {
                    const id = e?.media?.id
                    const g = e?.media?.genres as unknown as string[] | undefined
                    if (typeof id === "number" && Array.isArray(g)) {
                        map[id] = g
                    }
                }
            }
        } catch (_) {}
        return map
    }, [collection])

    return (
        <PageWrapper className="space-y-6">
            {/* Header with toolbar */}
            <div className="space-y-3">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold">Library</h1>
                        <p className="text-muted-foreground mt-1">
                            Manage your anime and manga collection
                        </p>
                    </div>
                </div>

                {/* Toolbar: search + genre chips */}
                <div className="flex flex-col gap-3">
                    <div className="flex items-center gap-2 max-w-xl">
                        <TextInput
                            value={search}
                            onValueChange={setSearch}
                            placeholder="Search manga by title..."
                            leftIcon={<FiSearch />}
                            className="w-full"
                            onKeyDown={(e) => {
                                if (e.key === "Escape") setSearch("")
                            }}
                        />
                        {!!search?.length && (
                            <IconButton
                                intent="white-basic"
                                size="sm"
                                icon={<FiX />}
                                onClick={() => setSearch("")}
                                aria-label="Clear search"
                            />
                        )}
                    </div>

                    {!!(genres && genres.length) && (
                        <MediaGenreSelector
                            items={genres.map((g) => ({
                                name: g,
                                isCurrent: selectedGenres.includes(g),
                                onClick: () =>
                                    setSelectedGenres((prev) =>
                                        prev.includes(g)
                                            ? prev.filter((x) => x !== g)
                                            : [...prev, g]
                                    ),
                            }))}
                        />
                    )}
                </div>
            </div>

            {/* Downloaded Series */}
            <div className="w-full space-y-6">
                <DownloadedMangaLibraryView
                    search={debouncedSearch}
                    selectedGenres={selectedGenres}
                    mediaGenresById={mediaGenresById}
                />
            </div>
        </PageWrapper>
    )
}
