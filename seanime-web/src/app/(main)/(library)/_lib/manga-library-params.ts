import { atom } from "jotai"
import { atomWithImmer } from "jotai-immer"
import { CollectionSorting } from "@/lib/helpers/filtering"

export type MangaLibraryParams = {
    genre?: string[]
    continueReadingOnly?: boolean
    sorting?: CollectionSorting<"manga">
}

export const __mangaLibrary_paramsAtom = atomWithImmer<MangaLibraryParams>({
    genre: [],
    continueReadingOnly: false,
    sorting: "SCORE_DESC",
})

export const __mangaLibrary_paramsInputAtom = atom(
    (get) => get(__mangaLibrary_paramsAtom),
    (get, set, update: MangaLibraryParams) => {
        set(__mangaLibrary_paramsAtom, update)
    },
)
