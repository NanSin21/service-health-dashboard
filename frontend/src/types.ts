export type ServiceStatus = 'UP' | 'DOWN';

export interface ServiceResult {
  name: string;
  status: ServiceStatus;
  details: string;
}

export interface HealthResponse {
  checkedAt: string;
  services: ServiceResult[];
}
