"use client"

import { EnMasseDownloader } from "@/app/(main)/_features/manga/en-masse-downloader"
import { NyaaCrawlerScreen } from "@/app/(main)/_features/nyaa-crawler/nyaa-crawler-screen"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs/tabs"

export default function EnMasseDownloaderPage() {
    return (
        <Tabs defaultValue="manga" className="w-full">
            <TabsList>
                <TabsTrigger value="manga">Manga</TabsTrigger>
                <TabsTrigger value="anime">Anime</TabsTrigger>
            </TabsList>
            <TabsContent value="manga" className="mt-4">
                <EnMasseDownloader />
            </TabsContent>
            <TabsContent value="anime" className="mt-4">
                <NyaaCrawlerScreen />
            </TabsContent>
        </Tabs>
    )
}
