export type ScorePeriodCode = string;
export type ExtraPointType = "add" | "deduct";

export interface DirectScoreItem {
  id: number;
  yearId: number;
  periodCode: ScorePeriodCode;
  moduleId: number;
  objectId: number;
  score: number;
  remark: string;
  inputBy: number;
  inputAt: number;
  updatedBy?: number;
  updatedAt?: number;
}

export interface CreateDirectScorePayload {
  yearId: number;
  periodCode: ScorePeriodCode;
  moduleId: number;
  objectId: number;
  score: number;
  remark?: string;
}

export interface UpdateDirectScorePayload {
  score: number;
  remark?: string;
}

export interface BatchDirectScoreEntry {
  objectId: number;
  score: number;
  remark?: string;
}

export interface BatchDirectScorePayload {
  yearId: number;
  periodCode: ScorePeriodCode;
  moduleId: number;
  overwrite: boolean;
  entries: BatchDirectScoreEntry[];
}

export interface BatchDirectScoreResult {
  created: number;
  updated: number;
  skipped: number;
}

export interface ListDirectScoresParams {
  yearId?: number;
  periodCode?: ScorePeriodCode;
  moduleId?: number;
  objectId?: number;
}

export interface ExtraPointItem {
  id: number;
  yearId: number;
  periodCode: ScorePeriodCode;
  objectId: number;
  pointType: ExtraPointType;
  points: number;
  reason: string;
  evidence: string;
  approvedBy?: number;
  approvedAt?: number;
  inputBy: number;
  inputAt: number;
  updatedBy?: number;
  updatedAt?: number;
}

export interface CreateExtraPointPayload {
  yearId: number;
  periodCode: ScorePeriodCode;
  objectId: number;
  pointType?: ExtraPointType;
  points: number;
  reason: string;
  evidence?: string;
  approve?: boolean;
}

export interface UpdateExtraPointPayload {
  pointType: ExtraPointType;
  points: number;
  reason: string;
  evidence?: string;
  approve?: boolean;
}

export interface ListExtraPointsParams {
  yearId?: number;
  periodCode?: ScorePeriodCode;
  objectId?: number;
  pointType?: ExtraPointType;
}
