"use client"
import { useSaveMangaLocally } from "@/api/hooks/manga_download.hooks"
import { Button } from "@/components/ui/button"
import React from "react"
import { MdCloudDownload } from "react-icons/md"
import { toast } from "sonner"

type SaveLocallyButtonProps = {
    mediaId: number
    provider: string
    disabled?: boolean
}

export function SaveLocallyButton(props: SaveLocallyButtonProps) {
    const { mediaId, provider, disabled } = props

    const { mutate: saveMangaLocally, isPending } = useSaveMangaLocally()

    const handleSaveLocally = React.useCallback(() => {
        if (!mediaId || !provider) {
            toast.error("Missing required information")
            return
        }

        saveMangaLocally({
            mediaId,
            provider,
        }, {
            onSuccess: () => {
                toast.success("Started downloading all chapters")
            },
            onError: (error) => {
                toast.error(`Failed to start download: ${error.message}`)
            },
        })
    }, [mediaId, provider, saveMangaLocally])

    return (
        <Button
            intent="primary-outline"
            size="sm"
            leftIcon={<MdCloudDownload />}
            onClick={handleSaveLocally}
            disabled={disabled || isPending}
            loading={isPending}
        >
            Save Locally
        </Button>
    )
}
