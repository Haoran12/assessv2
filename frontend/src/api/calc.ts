import { http } from "@/api/http";
import type {
  CalculatedModuleScoreItem,
  CalculatedScoreItem,
  ListCalculatedScoresParams,
  ListRankingsParams,
  RankingItem,
  RecalculatePayload,
  RecalculateResult,
} from "@/types/calc";

export async function recalculateScores(payload: RecalculatePayload): Promise<RecalculateResult> {
  const response = await http.post("/api/calc/recalculate", payload);
  return response.data?.data as RecalculateResult;
}

export async function listCalculatedScores(params: ListCalculatedScoresParams): Promise<CalculatedScoreItem[]> {
  const response = await http.get("/api/calc/scores", { params });
  return (response.data?.data?.items ?? []) as CalculatedScoreItem[];
}

export async function listCalculatedModuleScores(calculatedScoreId: number): Promise<CalculatedModuleScoreItem[]> {
  const response = await http.get(`/api/calc/scores/${calculatedScoreId}/modules`);
  return (response.data?.data?.items ?? []) as CalculatedModuleScoreItem[];
}

export async function listRankings(params: ListRankingsParams): Promise<RankingItem[]> {
  const response = await http.get("/api/calc/rankings", { params });
  return (response.data?.data?.items ?? []) as RankingItem[];
}
