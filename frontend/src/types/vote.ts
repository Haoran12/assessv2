import type { ScorePeriodCode } from "@/types/score";

export type VoteTaskStatus = "pending" | "completed" | "expired";
export type VoteGradeOption = "excellent" | "good" | "average" | "poor";

export interface GenerateVoteTasksPayload {
  yearId: number;
  periodCode: ScorePeriodCode;
  moduleId: number;
  objectIds?: number[];
}

export interface GenerateVoteTasksResult {
  created: number;
  skipped: number;
  groupCount: number;
  objectCount: number;
  voterCount: number;
}

export interface VoteTaskItem {
  id: number;
  yearId: number;
  periodCode: ScorePeriodCode;
  voteGroupId: number;
  objectId: number;
  voterId: number;
  status: VoteTaskStatus;
  completedAt?: number;
  createdBy?: number;
  createdAt: number;
  updatedAt: number;
  moduleId: number;
  moduleName: string;
  groupCode: string;
  groupName: string;
  gradeOption?: VoteGradeOption;
  remark?: string;
  votedAt?: number;
}

export interface ListVoteTasksParams {
  yearId?: number;
  periodCode?: ScorePeriodCode;
  moduleId?: number;
  objectId?: number;
  voterId?: number;
  status?: VoteTaskStatus;
  mine?: boolean;
}

export interface VoteRecordPayload {
  gradeOption: VoteGradeOption;
  remark?: string;
}

export interface VoteRecordResult {
  task: {
    id: number;
    yearId: number;
    periodCode: ScorePeriodCode;
    voteGroupId: number;
    objectId: number;
    voterId: number;
    status: VoteTaskStatus;
    completedAt?: number;
    createdBy?: number;
    createdAt: number;
    updatedAt: number;
  };
  record: {
    id: number;
    taskId: number;
    gradeOption: VoteGradeOption;
    remark: string;
    votedAt: number;
    createdAt: number;
    updatedAt: number;
  };
}

export interface VoteStatisticsFilter {
  yearId: number;
  periodCode: ScorePeriodCode;
  moduleId: number;
  objectId?: number;
}

export interface VoteGroupStatistics {
  voteGroupId: number;
  groupCode: string;
  groupName: string;
  totalTasks: number;
  completedTasks: number;
  pendingTasks: number;
  expiredTasks: number;
  gradeCounts: Record<VoteGradeOption, number>;
}

export interface VoteStatistics {
  totalTasks: number;
  completedTasks: number;
  pendingTasks: number;
  expiredTasks: number;
  completionRate: number;
  groupStatistics: VoteGroupStatistics[];
}
