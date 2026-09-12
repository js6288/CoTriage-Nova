import { http } from './http';

export type DeviceStatus = 'Ready' | 'NotReady' | 'Unknown';
export type CollectorStatus =
  | 'Running'
  | 'Pending'
  | 'CrashLoopBackOff'
  | 'ImagePullBackOff'
  | 'NotFound'
  | 'Unknown';

export interface DeviceItem {
  node_name: string;
  equipment: string;
  site: string;
  status: DeviceStatus;
  collector_status: CollectorStatus;
  last_event_time: string | null;
  ingest_rate_1m: number;
}

export interface ListDevicesResponse {
  items: DeviceItem[];
  total: number;
  generated_at: string;
}

export interface ListDevicesQuery {
  site?: string;
  equipment?: string;
  status?: DeviceStatus;
  limit?: number;
  offset?: number;
}

export async function listDevices(params: ListDevicesQuery = {}): Promise<ListDevicesResponse> {
  const res = await http.get<ListDevicesResponse>('/api/v1/devices', { params });
  return res.data;
}
