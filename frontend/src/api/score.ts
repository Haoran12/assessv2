import { http } from "@/api/http";
import type {
  BatchDirectScorePayload,
  BatchDirectScoreResult,
  CreateDirectScorePayload,
  CreateExtraPointPayload,
  DirectScoreItem,
  ExtraPointItem,
  ListDirectScoresParams,
  ListExtraPointsParams,
  UpdateDirectScorePayload,
  UpdateExtraPointPayload,
} from "@/types/score";

export async function listDirectScores(params: ListDirectScoresParams): Promise<DirectScoreItem[]> {
  const response = await http.get("/api/scores/direct", { params });
  return (response.data?.data?.items ?? []) as DirectScoreItem[];
}

export async function createDirectScore(payload: CreateDirectScorePayload): Promise<DirectScoreItem> {
  const response = await http.post("/api/scores/direct", payload);
  return response.data?.data as DirectScoreItem;
}

export async function batchUpsertDirectScores(payload: BatchDirectScorePayload): Promise<BatchDirectScoreResult> {
  const response = await http.post("/api/scores/direct/batch", payload);
  return response.data?.data as BatchDirectScoreResult;
}

export async function updateDirectScore(scoreId: number, payload: UpdateDirectScorePayload): Promise<DirectScoreItem> {
  const response = await http.put(`/api/scores/direct/${scoreId}`, payload);
  return response.data?.data as DirectScoreItem;
}

export async function deleteDirectScore(scoreId: number): Promise<void> {
  await http.delete(`/api/scores/direct/${scoreId}`);
}

export async function listExtraPoints(params: ListExtraPointsParams): Promise<ExtraPointItem[]> {
  const response = await http.get("/api/scores/extra", { params });
  return (response.data?.data?.items ?? []) as ExtraPointItem[];
}

export async function createExtraPoint(payload: CreateExtraPointPayload): Promise<ExtraPointItem> {
  const response = await http.post("/api/scores/extra", payload);
  return response.data?.data as ExtraPointItem;
}

export async function updateExtraPoint(extraPointId: number, payload: UpdateExtraPointPayload): Promise<ExtraPointItem> {
  const response = await http.put(`/api/scores/extra/${extraPointId}`, payload);
  return response.data?.data as ExtraPointItem;
}

export async function approveExtraPoint(extraPointId: number): Promise<ExtraPointItem> {
  const response = await http.post(`/api/scores/extra/${extraPointId}/approve`);
  return response.data?.data as ExtraPointItem;
}

export async function deleteExtraPoint(extraPointId: number): Promise<void> {
  await http.delete(`/api/scores/extra/${extraPointId}`);
}
