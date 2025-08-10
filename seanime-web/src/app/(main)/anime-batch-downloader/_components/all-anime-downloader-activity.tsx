"use client"

import React from "react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Separator } from "@/components/ui/separator"
import { AllAnimeDownloadJob } from "../_lib/use-all-anime-downloader"
import { AllAnimeDownloaderProgress } from "./all-anime-downloader-progress"
import { AllAnimeDownloaderStats } from "./all-anime-downloader-stats"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { BiListUl } from "react-icons/bi"

interface ActivityProps {
  job: AllAnimeDownloadJob
  onCancelDownload: () => Promise<void>
}

export function AllAnimeDownloaderActivity({ job, onCancelDownload }: ActivityProps) {
  return (
    <Tabs defaultValue="progress" className="space-y-4">
      <TabsList className="grid w-full grid-cols-3">
        <TabsTrigger value="progress">Progress</TabsTrigger>
        <TabsTrigger value="stats">Stats</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>

      <TabsContent value="progress">
        <AllAnimeDownloaderProgress job={job} onCancelDownload={onCancelDownload} />
      </TabsContent>

      <TabsContent value="stats">
        {job?.statistics && <AllAnimeDownloaderStats statistics={job.statistics} />}
      </TabsContent>

      <TabsContent value="logs">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <BiListUl className="w-4 h-4" />
              Activity Log
              <Badge className="ml-2" intent="gray">{job?.logs?.length ?? 0}</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {(!job?.logs || job.logs.length === 0) ? (
              <div className="text-sm text-muted-foreground">No log entries yet.</div>
            ) : (
              <div className="space-y-3">
                {job.logs.map((entry, idx) => (
                  <div key={idx} className="p-3 border rounded-md">
                    <div className="flex items-center justify-between text-sm">
                      <div className="font-medium">
                        {entry.animeTitle}
                      </div>
                      <Badge intent={entry.status === 'failed' ? 'alert' : (entry.status === 'success' ? 'success' : 'gray')}>
                        {entry.status}
                      </Badge>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground break-all">
                      Query: <span className="font-mono">{entry.query || '—'}</span>
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {entry.message}
                    </div>
                    <div className="mt-1 text-[10px] text-muted-foreground">
                      {new Date(entry.time).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  )
}
