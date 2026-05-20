import { createServiceClient } from '@/lib/client'
import { BugReportService } from '@/lib/gen/bugreport/v1/bugreport_connect'

export function useCreateBugReport() {
  const client = createServiceClient(BugReportService)

  return (
    title: string,
    description: string,
    page: string,
    consoleLogs: string,
    wsLog: string
  ) =>
    client.createBugReport({
      title,
      description,
      page,
      consoleLogs,
      wsLog
    })
}
