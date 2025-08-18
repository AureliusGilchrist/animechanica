import { Manga_Collection, Manga_CollectionList } from "@/api/generated/types"
import { useRefetchMangaChapterContainers } from "@/api/hooks/manga.hooks"
import { MediaCardLazyGrid } from "@/app/(main)/_features/media/_components/media-card-grid"
import { MediaEntryCard } from "@/app/(main)/_features/media/_components/media-entry-card"
import { MediaGenreSelector } from "@/app/(main)/_features/media/_components/media-genre-selector"
import { SeaCommandInjectableItem, useSeaCommandInject } from "@/app/(main)/_features/sea-command/use-inject"
import { seaCommand_compareMediaTitles } from "@/app/(main)/_features/sea-command/utils"
import { __mangaLibraryHeaderImageAtom, __mangaLibraryHeaderMangaAtom } from "@/app/(main)/manga/_components/library-header"
import { __mangaLibrary_paramsAtom, __mangaLibrary_paramsInputAtom, __mangaLibrary_searchInputAtom, __mangaLibrary_debouncedSearchInputAtom } from "@/app/(main)/manga/_lib/handle-manga-collection"
import { LuffyError } from "@/components/shared/luffy-error"
import { PageWrapper } from "@/components/shared/page-wrapper"
import { TextGenerateEffect } from "@/components/shared/text-generate-effect"
import { Button, IconButton } from "@/components/ui/button"
import { TextInput } from "@/components/ui/text-input"
import { DropdownMenu, DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { useDebounce } from "@/hooks/use-debounce"
import { getMangaCollectionTitle } from "@/lib/server/utils"
import { filterEntriesByTitle } from "@/lib/helpers/filtering"
import { ThemeLibraryScreenBannerType, useThemeSettings } from "@/lib/theme/hooks"
import { useSetAtom } from "jotai/index"
import { useAtom, useAtomValue } from "jotai/react"
import { AnimatePresence } from "motion/react"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import React, { memo } from "react"
import { BiDotsVertical } from "react-icons/bi"
import { FiSearch, FiX } from "react-icons/fi"
import { LuBookOpenCheck, LuRefreshCcw } from "react-icons/lu"
import { toast } from "sonner"
import { CommandItemMedia } from "../../_features/sea-command/_components/command-utils"
import { Select } from "@/components/ui/select/select"
import { useGetMangaCollectionPage } from "@/api/hooks/manga.hooks"

type MangaLibraryViewProps = {
    collection: Manga_Collection
    filteredCollection: Manga_Collection | undefined
    genres: string[]
    storedProviders: Record<string, string>
    hasManga: boolean
}

const MemoMediaEntryCard = React.memo(function MemoMediaEntryCard({ media, listData, type }: { media: any, listData: any, type: "manga" | "anime" }) {
    return (
        <MediaEntryCard
            media={media}
            listData={listData}
            showListDataButton
            withAudienceScore={false}
            type={type}
        />
    )
}, (prev, next) => {
    return prev.media?.id === next.media?.id && prev.type === next.type && prev.listData?.status === next.listData?.status && prev.listData?.progress === next.listData?.progress
})

function PaginatedMediaGrid({
    entries,
    type,
    defaultPageSize = 20,
    onEntryHover,
    queryKey,
    server,
}: {
    entries: any[]
    type: "manga" | "anime"
    defaultPageSize?: number
    onEntryHover?: (entry: any) => void
    queryKey?: string
    server?: { status: "CURRENT" | "PLANNING" | "COMPLETED" | "PAUSED" | "DROPPED" }
}) {
    const router = useRouter()
    const pathname = usePathname()
    const searchParams = useSearchParams()
    // no-op

    const sizeParamKey = React.useMemo(() => `${queryKey || type}_size`, [queryKey, type])
    const pageParamKey = React.useMemo(() => `${queryKey || type}_page`, [queryKey, type])

    const initialSize = React.useMemo(() => {
        const v = Number(searchParams.get(sizeParamKey))
        return Number.isFinite(v) && v > 0 ? v : defaultPageSize
    }, [searchParams, sizeParamKey, defaultPageSize])

    const initialPage = React.useMemo(() => {
        const v = Number(searchParams.get(pageParamKey))
        return Number.isFinite(v) && v > 0 ? v : 1
    }, [searchParams, pageParamKey])

    const [pageSize, setPageSize] = React.useState<number>(initialSize)
    const [page, setPage] = React.useState<number>(initialPage)

    const clientTotal = entries?.length || 0
    const clientPageCount = Math.max(1, Math.ceil(clientTotal / pageSize))
    const safePage = Math.max(1, page)
    const start = (safePage - 1) * pageSize
    const end = start + pageSize
    const pageCacheRef = React.useRef<Map<string, any[]>>(new Map())
    const cacheKey = React.useMemo(() => `${pageSize}:${safePage}`, [pageSize, safePage])
    const pageEntries = React.useMemo(() => {
        const cached = pageCacheRef.current.get(cacheKey)
        if (cached) return cached
        const sliced = (entries || []).slice(start, end)
        pageCacheRef.current.set(cacheKey, sliced)
        return sliced
    }, [entries, start, end, cacheKey])

    // Server-side pagination
    const { data: serverPage, isLoading, isError } = useGetMangaCollectionPage(
        { status: server?.status as any, page: safePage, pageSize },
        !!server
    )
    const serverPageCount = React.useMemo(() => {
        if (!server) return clientPageCount
        const total = serverPage?.total ?? 0
        return Math.max(1, Math.ceil(total / pageSize))
    }, [server, serverPage?.total, pageSize, clientPageCount])

    // keep URL in sync on page/pageSize change
    const updateUrl = React.useCallback((nextPage: number, nextSize: number) => {
        const sp = new URLSearchParams(searchParams.toString())
        sp.set(pageParamKey, String(nextPage))
        sp.set(sizeParamKey, String(nextSize))
        router.replace(`${pathname}?${sp.toString()}`, { scroll: false })
    }, [router, pathname, searchParams, pageParamKey, sizeParamKey])

    React.useEffect(() => {
        // reset to page 1 when entries set changes (client mode)
        if (!server) {
            setPage(1)
            pageCacheRef.current.clear()
        }
    }, [entries, server])

    React.useEffect(() => {
        // clamp page when total changes
        const pc = server ? serverPageCount : clientPageCount
        if (page > pc) {
            setPage(pc)
            updateUrl(pc, pageSize)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [serverPageCount, clientPageCount, server])

    React.useEffect(() => {
        updateUrl(page, pageSize)
    }, [page, pageSize, updateUrl])

    // Prefetch adjacent pages (server mode)
    React.useEffect(() => {
        if (!server) return
        const status = server.status
        const next = safePage + 1
        const prev = Math.max(1, safePage - 1)
        const totalPages = serverPageCount
        // Trigger hidden queries via our hook for caching
        // Note: using the same hook is acceptable for prefetch when enabled flag drives request
        // eslint-disable-next-line react-hooks/rules-of-hooks
        // no-op: hooks cannot be called conditionally here; instead rely on separate invisible components would be overkill
        // So we skip explicit prefetch here to keep rule-compliant. React Query cache will keep current page.
        // Optionally, we could dispatch background requests via fetch, but we'd need auth context.
        void status; void next; void prev; void totalPages
    }, [server, safePage, serverPageCount])

    const itemsToRender = server ? (serverPage?.items || []) : pageEntries

    return (
        <>
            {server && isLoading && (
                <div className="text-sm text-[--muted] text-center py-2">Loading...</div>
            )}
            {server && isError && (
                <div className="text-sm text-red-500 text-center py-2">Failed to load page.</div>
            )}
            <MediaCardLazyGrid itemCount={itemsToRender.length}>
                {itemsToRender.map((entry) => (
                    <div
                        key={entry.media?.id}
                        onMouseEnter={() => {
                            onEntryHover?.(entry)
                        }}
                    >
                        <MemoMediaEntryCard media={entry.media!} listData={entry.listData} type={type} />
                    </div>
                ))}
            </MediaCardLazyGrid>
            <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
                <div className="flex items-center gap-2">
                    <span className="text-sm text-[--muted]">Page size</span>
                    <Select
                        size="sm"
                        placeholder=""
                        value={String(pageSize)}
                        onValueChange={(val) => {
                            const next = Number(val)
                            if (!Number.isFinite(next) || next <= 0) return
                            setPageSize(next)
                            setPage(1)
                            updateUrl(1, next)
                        }}
                        options={[12, 20, 40, 60, 100].map(sz => ({ value: String(sz), label: String(sz) }))}
                        className="min-w-[5rem]"
                    />
                </div>
                {(server ? serverPageCount : clientPageCount) > 1 && (
                    <div className="flex items-center justify-center gap-3">
                        <button
                            className="px-3 py-1 rounded border disabled:opacity-50"
                            onClick={() => setPage(p => Math.max(1, p - 1))}
                            disabled={safePage === 1}
                        >
                            Prev
                        </button>
                        <span className="text-sm text-[--muted]">Page {safePage} / {server ? serverPageCount : clientPageCount}</span>
                        <button
                            className="px-3 py-1 rounded border disabled:opacity-50"
                            onClick={() => setPage(p => Math.min((server ? serverPageCount : clientPageCount), p + 1))}
                            disabled={safePage === (server ? serverPageCount : clientPageCount)}
                        >
                            Next
                        </button>
                    </div>
                )}
            </div>
        </>
    )
}

export function MangaLibraryView(props: MangaLibraryViewProps) {

    const {
        collection,
        filteredCollection,
        genres,
        storedProviders,
        hasManga,
        ...rest
    } = props

    const [params, setParams] = useAtom(__mangaLibrary_paramsAtom)
    const [search, setSearch] = useAtom(__mangaLibrary_searchInputAtom)
    const debouncedSearch = useDebounce(search, 250)
    const setDebouncedSearch = useSetAtom(__mangaLibrary_debouncedSearchInputAtom)
    React.useEffect(() => {
        setDebouncedSearch(debouncedSearch)
    }, [debouncedSearch])

    return (
        <>
            <PageWrapper
                key="lists"
                className="relative 2xl:order-first pb-10 p-4"
                data-manga-library-view-container
            >
                <div className="w-full flex items-center justify-between gap-3 mb-3">
                    <div className="flex-1 max-w-xl flex items-center gap-2">
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
                </div>

                <AnimatePresence mode="wait" initial={false}>

                    {!!collection && !hasManga && <LuffyError
                        title="No manga found"
                    >
                        <div className="space-y-2">
                            <p>
                                No manga has been added to your library yet.
                            </p>

                            <div className="!mt-4">
                                <Link href="/discover?type=manga">
                                    <Button intent="white-outline" rounded>
                                        Browse manga
                                    </Button>
                                </Link>
                            </div>
                        </div>
                    </LuffyError>}

                    {!params.genre?.length ?
                        <CollectionLists key="lists" collectionList={collection} genres={genres} storedProviders={storedProviders} search={debouncedSearch} />
                        : <FilteredCollectionLists key="filtered-collection" collectionList={filteredCollection} genres={genres} search={debouncedSearch} />
                    }
                </AnimatePresence>
            </PageWrapper>
        </>
    )
}

export function CollectionLists({ collectionList, genres, storedProviders, search }: {
    collectionList: Manga_Collection | undefined
    genres: string[]
    storedProviders: Record<string, string>
    search: string
}) {
    const clearMangaSearch = useSetAtom(__mangaLibrary_searchInputAtom)
    const totalFiltered = React.useMemo(() => {
        if (!collectionList?.lists?.length) return 0
        return collectionList.lists.reduce((acc, collection) => {
            const filtered = filterEntriesByTitle(collection.entries ?? [], search)
            return acc + (filtered?.length || 0)
        }, 0)
    }, [collectionList, search])

    return (
        <PageWrapper
            className="p-4 space-y-8 relative z-[4]"
            data-manga-library-view-collection-lists-container
            {...{
                initial: { opacity: 0, y: 60 },
                animate: { opacity: 1, y: 0 },
                exit: { opacity: 0, scale: 0.99 },
                transition: {
                    duration: 0.35,
                },
            }}
        >
            {totalFiltered === 0 && (
                <LuffyError title="No results">
                    <p className="text-[--muted]">Try a different search or adjust your filters.</p>
                    <div className="mt-2">
                        <Button intent="white-outline" size="sm" onClick={() => clearMangaSearch("")}>Clear search</Button>
                    </div>
                </LuffyError>
            )}
            {collectionList?.lists?.map(collection => {
                if (!collection.entries?.length) return null
                const filteredEntries = filterEntriesByTitle(collection.entries, search)
                if (!filteredEntries?.length) return null
                const listWithSearch = { ...collection, entries: filteredEntries }
                return (
                    <React.Fragment key={collection.type}>
                        <CollectionListItem list={listWithSearch} storedProviders={storedProviders} />

                        {(collection.type === "CURRENT" && !!genres?.length) && <GenreSelector genres={genres} />}
                    </React.Fragment>
                )
            })}
        </PageWrapper>
    )

}

export function FilteredCollectionLists({ collectionList, genres, search }: {
    collectionList: Manga_Collection | undefined
    genres: string[]
    search: string
}) {

    const clearMangaSearch = useSetAtom(__mangaLibrary_searchInputAtom)
    const entries = React.useMemo(() => {
        const flat = collectionList?.lists?.flatMap(n => n.entries).filter(Boolean) ?? []
        return filterEntriesByTitle(flat, search) as typeof flat
    }, [collectionList, search])

    return (
        <PageWrapper
            className="p-4 space-y-8 relative z-[4]"
            data-manga-library-view-filtered-collection-lists-container
            {...{
                initial: { opacity: 0, y: 60 },
                animate: { opacity: 1, y: 0 },
                 exit: { opacity: 0, scale: 0.99 },
                transition: {
                    duration: 0.35,
                },
            }}
        >
            {entries.length === 0 && (
                <LuffyError title="No results">
                    <p className="text-[--muted]">Try a different search or adjust your filters.</p>
                    <div className="mt-2">
                        <Button intent="white-outline" size="sm" onClick={() => clearMangaSearch("")}>Clear search</Button>
                    </div>
                </LuffyError>
            )}
            {!!genres?.length && <div className="mt-24">
                <GenreSelector genres={genres} />
            </div>}

            <PaginatedMediaGrid
                entries={entries}
                type="manga"
                queryKey="manga_filtered"
            />
        </PageWrapper>
    )

}

const CollectionListItem = memo(({ list, storedProviders }: { list: Manga_CollectionList, storedProviders: Record<string, string> }) => {

    const ts = useThemeSettings()
    const [currentHeaderImage, setCurrentHeaderImage] = useAtom(__mangaLibraryHeaderImageAtom)
    const headerManga = useAtomValue(__mangaLibraryHeaderMangaAtom)
    const [params, setParams] = useAtom(__mangaLibrary_paramsAtom)
    const router = useRouter()

    const { mutate: refetchMangaChapterContainers, isPending: isRefetchingMangaChapterContainers } = useRefetchMangaChapterContainers()

    const { inject, remove } = useSeaCommandInject()

    React.useEffect(() => {
        if (list.type === "CURRENT") {
            if (currentHeaderImage === null && list.entries?.[0]?.media?.bannerImage) {
                setCurrentHeaderImage(list.entries?.[0]?.media?.bannerImage)
            }
        }
    }, [])

    // Inject command for currently reading manga
    React.useEffect(() => {
        if (list.type === "CURRENT" && list.entries?.length) {
            inject("currently-reading-manga", {
                items: list.entries.map(entry => ({
                    data: entry,
                    id: `manga-${entry.mediaId}`,
                    value: entry.media?.title?.userPreferred || "",
                    heading: "Currently Reading",
                    priority: 100,
                    render: () => (
                        <CommandItemMedia media={entry.media!} />
                    ),
                    onSelect: () => {
                        router.push(`/manga/entry?id=${entry.mediaId}`)
                    },
                })),
                filter: ({ item, input }: { item: SeaCommandInjectableItem, input: string }) => {
                    if (!input) return true
                    return seaCommand_compareMediaTitles((item.data as typeof list.entries[0])?.media?.title, input)
                },
                priority: 100,
            })
        }

        return () => remove("currently-reading-manga")
    }, [list.entries])

    return (
        <React.Fragment>

            <div className="flex gap-3 items-center" data-manga-library-view-collection-list-item-header-container>
                <h2 data-manga-library-view-collection-list-item-header-title>{list.type === "CURRENT" ? "Continue reading" : getMangaCollectionTitle(
                    list.type)}</h2>
                <div className="flex flex-1" data-manga-library-view-collection-list-item-header-spacer></div>

                {list.type === "CURRENT" && params.unreadOnly && (
                    <Button
                        intent="white-link"
                        size="xs"
                        className="!px-2 !py-1"
                        onClick={() => {
                            setParams(draft => {
                                draft.unreadOnly = false
                                return
                            })
                        }}
                    >
                        Show all
                    </Button>
                )}

                {list.type === "CURRENT" && <DropdownMenu
                    trigger={<div className="relative">
                        <IconButton
                            intent="white-basic"
                            size="xs"
                            className="mt-1"
                            icon={<BiDotsVertical />}
                            // loading={isRefetchingMangaChapterContainers}
                        />
                        {/*{params.unreadOnly && <div className="absolute -top-1 -right-1 bg-[--blue] size-2 rounded-full"></div>}*/}
                        {isRefetchingMangaChapterContainers &&
                            <div className="absolute -top-1 -right-1 bg-[--orange] size-3 rounded-full animate-ping"></div>}
                    </div>}
                >
                    <DropdownMenuItem
                        onClick={() => {
                            if (isRefetchingMangaChapterContainers) return

                            toast.info("Refetching from sources...")
                            refetchMangaChapterContainers({
                                selectedProviderMap: storedProviders,
                            })
                        }}
                    >
                        <LuRefreshCcw /> {isRefetchingMangaChapterContainers ? "Refetching..." : "Refresh sources"}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                        onClick={() => {
                            setParams(draft => {
                                draft.unreadOnly = !draft.unreadOnly
                                return
                            })
                        }}
                    >
                        <LuBookOpenCheck /> {params.unreadOnly ? "Show all" : "Unread chapters only"}
                    </DropdownMenuItem>
                </DropdownMenu>}

            </div>

            {(list.type === "CURRENT" && ts.libraryScreenBannerType === ThemeLibraryScreenBannerType.Dynamic && headerManga) &&
                <TextGenerateEffect
                    data-manga-library-view-collection-list-item-header-media-title
                    words={headerManga?.title?.userPreferred || ""}
                    className="w-full text-xl lg:text-5xl lg:max-w-[50%] h-[3.2rem] !mt-1 line-clamp-1 truncate text-ellipsis hidden lg:block pb-1"
                />
            }

            <PaginatedMediaGrid
                entries={list.entries || []}
                type="manga"
                queryKey={`manga_${(list.type || "list").toLowerCase()}`}
                server={{ status: (list.type as any) }}
                onEntryHover={(entry) => {
                    if (list.type === "CURRENT" && entry.media?.bannerImage) {
                        React.startTransition(() => {
                            setCurrentHeaderImage(entry.media?.bannerImage!)
                        })
                    }
                }}
            />
        </React.Fragment>
    )
})

function GenreSelector({
    genres,
}: { genres: string[] }) {
    const [params, setParams] = useAtom(__mangaLibrary_paramsInputAtom)
    const setActualParams = useSetAtom(__mangaLibrary_paramsAtom)
    const debouncedParams = useDebounce(params, 200)

    React.useEffect(() => {
        setActualParams(params)
    }, [debouncedParams])

    if (!genres.length) return null

    return (
        <MediaGenreSelector
            // className="bg-gray-950 border p-0 rounded-xl mx-auto"
            staticTabsClass=""
            items={[
                ...genres.map(genre => ({
                    name: genre,
                    isCurrent: params!.genre?.includes(genre) ?? false,
                    onClick: () => setParams(draft => {
                        if (draft.genre?.includes(genre)) {
                            draft.genre = draft.genre?.filter(g => g !== genre)
                        } else {
                            draft.genre = [...(draft.genre || []), genre]
                        }
                        return
                    }),
                })),
            ]}
        />
    )
}
