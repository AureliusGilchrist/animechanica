import { Anime_Entry } from "@/api/generated/types"
import { useUpdateLocalFiles } from "@/api/hooks/localfiles.hooks"
import { FilepathSelector } from "@/app/(main)/_features/media/_components/filepath-selector"
import { ConfirmationDialog, useConfirmationDialog } from "@/components/shared/confirmation-dialog"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { atom } from "jotai/index"
import { useAtom } from "jotai/react"
import React from "react"

export type AnimeEntryMatchFilesModalProps = {
    entry: Anime_Entry
}

export const __animeEntryMatchFilesModalIsOpenAtom = atom(false)

export function AnimeEntryMatchFilesModal({ entry }: AnimeEntryMatchFilesModalProps) {

    const [open, setOpen] = useAtom(__animeEntryMatchFilesModalIsOpenAtom)

    return (
        <Modal
            open={open}
            onOpenChange={() => setOpen(false)}
            contentClass="max-w-2xl"
            title={<span>Select files to match to this anime</span>}
            titleClass="text-center"
        >
            <Content entry={entry} />
        </Modal>
    )
}

function Content({ entry }: { entry: Anime_Entry }) {
    const [open, setOpen] = useAtom(__animeEntryMatchFilesModalIsOpenAtom)

    const [filepaths, setFilepaths] = React.useState<string[]>([])

    const media = entry.media

    // Preselect unmatched files when available
    React.useEffect(() => {
        const all = entry.localFiles?.map(f => f.path) ?? []
        const unmatched = entry.localFiles?.filter(f => !f.mediaId || f.mediaId === 0)?.map(f => f.path) ?? []
        setFilepaths(unmatched.length > 0 ? unmatched : all)
    }, [entry.localFiles])

    const { mutate: updateFiles, isPending: isMatching } = useUpdateLocalFiles()

    const confirmMatch = useConfirmationDialog({
        title: "Match files to this anime",
        description: "This will set the MediaId on the selected files and lock them so scans won't override your choice.",
        onConfirm: () => {
            if (filepaths.length === 0 || !entry.mediaId) return

            updateFiles({
                paths: filepaths,
                action: "match",
                mediaId: entry.mediaId,
            }, {
                onSuccess: () => {
                    setOpen(false)
                },
            })
        },
    })

    if (!media) return null

    return (
        <div className="space-y-2 mt-2">
            <FilepathSelector
                className="max-h-96"
                filepaths={filepaths}
                allFilepaths={entry.localFiles?.map(n => n.path) ?? []}
                onFilepathSelected={setFilepaths}
                showFullPath
            />
            <div className="text-xs text-[--muted]">
                Selected files will be force-matched to "{media?.title?.userPreferred ?? media?.title?.romaji ?? media?.title?.english}" and locked.
            </div>
            <div className="flex justify-end gap-2 mt-2">
                <Button
                    intent="primary"
                    onClick={() => confirmMatch.open()}
                    loading={isMatching}
                >
                    Match to this anime
                </Button>
                <Button
                    intent="white"
                    onClick={() => setOpen(false)}
                    disabled={isMatching}
                >
                    Cancel
                </Button>
            </div>
            <ConfirmationDialog {...confirmMatch} />
        </div>
    )
}
