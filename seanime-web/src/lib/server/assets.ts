import { getServerBaseUrl } from "@/api/client/server-url"

export function getImageUrl(path: string) {
    if (path.startsWith("{{LOCAL_ASSETS}}")) {
        const replaced = path.replace("{{LOCAL_ASSETS}}", "offline-assets")
        // Append deterministic cache-busting query tied to the asset path
        const v = encodeURIComponent(replaced)
        return `${getServerBaseUrl()}/${replaced}?v=${v}`
    }

    return path
}

export function getAssetUrl(path: string) {
    let p = path.replaceAll("\\", "/")

    if (p.startsWith("/")) {
        p = p.substring(1)
    }

    p = encodeURIComponent(p).replace(/\(/g, "%28").replace(/\)/g, "%29")

    if (p.startsWith("{{LOCAL_ASSETS}}")) {
        const replaced = p.replace("{{LOCAL_ASSETS}}", "offline-assets")
        const v = encodeURIComponent(replaced)
        return `${getServerBaseUrl()}/${replaced}?v=${v}`
    }

    return `${getServerBaseUrl()}/assets/${p}`
}

export function legacy_getAssetUrl(path: string) {
    let p = path.replaceAll("\\", "/")

    if (p.startsWith("/")) {
        p = p.substring(1)
    }

    p = encodeURIComponent(p).replace(/\(/g, "%28").replace(/\)/g, "%29")

    return `${getServerBaseUrl()}/assets/${p}`
}
