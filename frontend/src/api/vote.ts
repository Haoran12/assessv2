import { http } from "@/api/http";
import type {
  GenerateVoteTasksPayload,
  GenerateVoteTasksResult,
  ListVoteTasksParams,
  VoteRecordPayload,
  VoteRecordResult,
  VoteStatistics,
  VoteStatisticsFilter,
  VoteTaskItem,
} from "@/types/vote";

export async function generateVoteTasks(payload: GenerateVoteTasksPayload): Promise<GenerateVoteTasksResult> {
  const response = await http.post("/api/votes/tasks/generate", payload);
  return response.data?.data as GenerateVoteTasksResult;
}

export async function listVoteTasks(params: ListVoteTasksParams): Promise<VoteTaskItem[]> {
  const response = await http.get("/api/votes/tasks", { params });
  return (response.data?.data?.items ?? []) as VoteTaskItem[];
}

export async function saveVoteDraft(taskId: number, payload: VoteRecordPayload): Promise<VoteRecordResult> {
  const response = await http.post(`/api/votes/tasks/${taskId}/draft`, payload);
  return response.data?.data as VoteRecordResult;
}

export async function submitVote(taskId: number, payload: VoteRecordPayload): Promise<VoteRecordResult> {
  const response = await http.post(`/api/votes/tasks/${taskId}/submit`, payload);
  return response.data?.data as VoteRecordResult;
}

export async function resetVoteTask(taskId: number): Promise<void> {
  await http.post(`/api/votes/tasks/${taskId}/reset`);
}

export async function getVoteStatistics(params: VoteStatisticsFilter): Promise<VoteStatistics> {
  const response = await http.get("/api/votes/statistics", { params });
  return response.data?.data as VoteStatistics;
}
