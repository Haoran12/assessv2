import type { AssessmentObjectCategory, AssessmentObjectType } from "@/types/assessment";
import type { ScorePeriodCode } from "@/types/score";

export type CalcTriggerMode = "auto" | "manual";
export type RankingScope = "overall" | "parent_object";

export interface RecalculatePayload {
  yearId: number;
  periodCode: ScorePeriodCode;
  objectIds?: number[];
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
  targetType?: string;
  targetId?: number;
}

export interface RecalculateResult {
  yearId: number;
  periodCode: ScorePeriodCode;
  triggerMode: CalcTriggerMode;
  totalObjects: number;
  calculatedObjects: number;
  skippedObjects: number;
  durationMs: number;
}

export interface ListCalculatedScoresParams {
  yearId?: number;
  periodCode?: ScorePeriodCode;
  objectId?: number;
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
}

export interface CalculatedScoreItem {
  id: number;
  yearId: number;
  periodCode: ScorePeriodCode;
  objectId: number;
  ruleId: number;
  weightedScore: number;
  extraPoints: number;
  finalScore: number;
  rankBasis: string;
  detailJson: string;
  triggerMode: CalcTriggerMode;
  triggeredBy?: number;
  calculatedAt: number;
  createdAt: number;
  updatedAt: number;
  objectName: string;
  objectType: AssessmentObjectType;
  objectCategory: AssessmentObjectCategory;
  parentObjectId?: number;
  overallRank?: number;
}

export interface CalculatedModuleScoreItem {
  id: number;
  calculatedScoreId: number;
  moduleId: number;
  moduleCode: string;
  moduleKey: string;
  moduleName: string;
  sortOrder: number;
  rawScore: number;
  weightedScore: number;
  scoreDetail: string;
  createdAt: number;
  updatedAt: number;
}

export interface ListRankingsParams {
  yearId?: number;
  periodCode?: ScorePeriodCode;
  scope?: RankingScope;
  scopeKey?: string;
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
}

export interface RankingItem {
  id: number;
  yearId: number;
  periodCode: ScorePeriodCode;
  objectId: number;
  objectType: AssessmentObjectType;
  objectCategory: AssessmentObjectCategory;
  rankingScope: RankingScope;
  scopeKey: string;
  rankNo: number;
  score: number;
  tieBreakKey: string;
  calculatedScoreId: number;
  createdAt: number;
  updatedAt: number;
  objectName: string;
}
