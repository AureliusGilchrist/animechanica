"use client"
import { Anime_Entry } from "@/api/generated/types"
import { Badge } from "@/components/ui/badge"
import React from "react"

function normalize(path: string) {
    return (path || "").replace(/\\/g, "/")
}

function dirname(path: string) {
    const p = normalize(path)
    const idx = p.lastIndexOf("/")
    return idx >= 0 ? p.slice(0, idx) : ""
}

function commonPrefixDir(paths: string[]): string {
    if (paths.length === 0) return ""
    const parts = paths.map(p => normalize(dirname(p)).split("/").filter(Boolean))
    const minLen = Math.min(...parts.map(p => p.length))
    const prefix: string[] = []
    for (let i = 0; i < minLen; i++) {
        const seg = parts[0][i]
        if (parts.every(p => p[i] === seg)) prefix.push(seg)
        else break
    }
    return prefix.length ? "/" + prefix.join("/") : ""
}

export function AnimeEntryFolderIndicator({ entry }: { entry: Anime_Entry }) {
    const paths = React.useMemo(() => (entry.localFiles ?? []).map(f => f.path).filter(Boolean), [entry.localFiles])
    if (paths.length === 0) return null

    const root = commonPrefixDir(paths)

    // Compute immediate child folders under root
    const childFolders = new Set<string>()
    let hasRootFiles = false

    ;(entry.localFiles ?? []).forEach(f => {
        const full = normalize(f.path)
        const dir = dirname(full)
        let rel = root && full.startsWith(root) ? dir.slice(root.length) : dir
        rel = rel.replace(/^\/+/, "") // trim leading slashes
        if (!rel) {
            hasRootFiles = true
            return
        }
        const first = rel.split("/")[0]
        if (first) childFolders.add(first)
    })

    if (!hasRootFiles && childFolders.size === 0) return null

    const folders = Array.from(childFolders)

    return (
        <div className="flex flex-wrap items-center gap-2 text-xs" data-anime-entry-folder-indicator>
            <span className="text-[--muted]">Folders:</span>
            {hasRootFiles && <Badge intent="gray" title={root || "/"}>root</Badge>}
            {folders.map(name => (
                <Badge key={name} intent="gray" title={`${root ? root + "/" : "/"}${name}`}>
                    {name}
                </Badge>
            ))}
        </div>
    )
}
