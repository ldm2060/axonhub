import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

const SYSTEM_RUNTIME_OVERVIEW_QUERY = `
  query SystemRuntimeOverview {
    systemRuntimeOverview {
      collectedAt
      sampleIntervalSeconds
      retentionSeconds
      host {
        hostname
        os
        architecture
        platform
        platformVersion
        kernelVersion
        logicalCpus
        processId
        goVersion
        version
        serviceStartedAt
      }
      current {
        timestamp
        systemCpuPercent
        processCpuPercent
        memoryUsedPercent
        memoryUsedBytes
        memoryTotalBytes
        processRssBytes
        processHeapAllocBytes
        networkReceiveBytesPerSecond
        networkTransmitBytesPerSecond
        diskUsedPercent
        diskUsedBytes
        diskTotalBytes
        load1
        load5
        load15
        goroutines
        processThreads
        gcCount
        gcPauseMilliseconds
        serviceUptimeSeconds
      }
      stats {
        periodStart
        periodEnd
        sampleCount
        systemCpuAveragePercent
        systemCpuMaxPercent
        processCpuAveragePercent
        processCpuMaxPercent
        memoryUsedAveragePercent
        memoryUsedMaxPercent
        processRssAverageBytes
        processRssMaxBytes
        networkReceiveTotalBytes
        networkTransmitTotalBytes
        networkReceivePeakBytesPerSecond
        networkTransmitPeakBytesPerSecond
      }
      history {
        timestamp
        systemCpuPercent
        processCpuPercent
        memoryUsedPercent
        processRssBytes
        processHeapAllocBytes
        networkReceiveBytesPerSecond
        networkTransmitBytesPerSecond
      }
    }
  }
`;

export interface SystemRuntimeHost {
  hostname: string;
  os: string;
  architecture: string;
  platform: string;
  platformVersion: string;
  kernelVersion: string;
  logicalCpus: number;
  processId: number;
  goVersion: string;
  version: string;
  serviceStartedAt: string;
}

export interface SystemRuntimeSample {
  timestamp: string;
  systemCpuPercent: number;
  processCpuPercent: number;
  memoryUsedPercent: number;
  memoryUsedBytes: number;
  memoryTotalBytes: number;
  processRssBytes: number;
  processHeapAllocBytes: number;
  networkReceiveBytesPerSecond: number;
  networkTransmitBytesPerSecond: number;
  diskUsedPercent: number;
  diskUsedBytes: number;
  diskTotalBytes: number;
  load1: number;
  load5: number;
  load15: number;
  goroutines: number;
  processThreads: number;
  gcCount: number;
  gcPauseMilliseconds: number;
  serviceUptimeSeconds: number;
}

export interface SystemRuntimeStats {
  periodStart: string;
  periodEnd: string;
  sampleCount: number;
  systemCpuAveragePercent: number;
  systemCpuMaxPercent: number;
  processCpuAveragePercent: number;
  processCpuMaxPercent: number;
  memoryUsedAveragePercent: number;
  memoryUsedMaxPercent: number;
  processRssAverageBytes: number;
  processRssMaxBytes: number;
  networkReceiveTotalBytes: number;
  networkTransmitTotalBytes: number;
  networkReceivePeakBytesPerSecond: number;
  networkTransmitPeakBytesPerSecond: number;
}

export interface SystemRuntimeOverview {
  collectedAt: string;
  sampleIntervalSeconds: number;
  retentionSeconds: number;
  host: SystemRuntimeHost;
  current: SystemRuntimeSample;
  stats: SystemRuntimeStats;
  history: SystemRuntimeSample[];
}

export function useSystemRuntimeOverview() {
  return useQuery({
    queryKey: ['systemRuntimeOverview'],
    queryFn: async () => {
      const data = await graphqlRequest<{ systemRuntimeOverview: SystemRuntimeOverview }>(SYSTEM_RUNTIME_OVERVIEW_QUERY);
      return data.systemRuntimeOverview;
    },
    refetchInterval: 10_000,
    staleTime: 4_000,
  });
}
