import React from "react"
import { useAtom } from "jotai"
import { MediaGenreSelector } from "@/app/(main)/_features/media/_components/media-genre-selector"
import { __mangaLibrary_paramsAtom, MangaLibraryParams } from "@/app/(main)/(library)/_lib/manga-library-params"
import { PageWrapper } from "@/components/shared/page-wrapper"

type MangaGenreSelectorProps = {
    genres: string[]
}

export function MangaGenreSelector({ genres }: MangaGenreSelectorProps) {
    const [params, setParams] = useAtom(__mangaLibrary_paramsAtom)

    if (!genres.length) return null

    return (
        <PageWrapper className="space-y-3 lg:space-y-6 relative z-[4]" data-manga-library-genre-selector-container>
            <MediaGenreSelector
                items={[
                    ...genres.map(genre => ({
                        name: genre,
                        isCurrent: params.genre?.includes(genre) ?? false,
                        onClick: () => setParams((draft: MangaLibraryParams) => {
                            if (draft.genre?.includes(genre)) {
                                draft.genre = draft.genre?.filter((g: string) => g !== genre)
                            } else {
                                draft.genre = [...(draft.genre || []), genre]
                            }
                            return
                        }),
                    })),
                ]}
            />
        </PageWrapper>
    )
}
