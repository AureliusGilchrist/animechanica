import { AppLayoutStack } from "@/components/ui/app-layout"
import { PageWrapper } from "@/components/shared/page-wrapper"

export default function AnimeBatchDownloaderLayout({ children }: { children: React.ReactNode }) {
    return (
        <PageWrapper className="p-4 sm:p-8">
            <AppLayoutStack>
                {children}
            </AppLayoutStack>
        </PageWrapper>
    )
}
