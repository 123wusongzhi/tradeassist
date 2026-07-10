import { getJSON } from '@/services/request';

export type ConfigStatusItem = {
  key: string;
  title: string;
  status: string;
  summary?: string;
  impactScope?: string;
  nextAction?: string;
  settingsUrl?: string;
};

export type ProjectPhase = {
  phase: string;
  statusLines: string[];
  finalAcceptance: string;
  productionReady: boolean;
  tagDeferred: boolean;
  grayReleaseAllowed: boolean;
  infrastructureFoundationReady: boolean;
};

export type ConfigStatusOverview = {
  generatedAt: string;
  projectPhase?: ProjectPhase;
  items: ConfigStatusItem[];
  demoData: ConfigStatusItem;
};

export async function fetchConfigStatusOverview() {
  return getJSON<ConfigStatusOverview>('/api/v1/settings/config-status');
}
