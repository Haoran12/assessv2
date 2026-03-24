import { http } from "@/api/http";
import type {
  AssessmentObjectCandidateItem,
  AssessmentSessionDetail,
  AssessmentSessionItem,
  AssessmentSessionObjectItem,
  CreateAssessmentSessionPayload,
  UpdateAssessmentModuleScoresPayload,
  UpdateAssessmentObjectGroupsPayload,
  UpdateAssessmentObjectsPayload,
  UpdateAssessmentPeriodsPayload,
  UpdateAssessmentSessionPayload,
  UpdateAssessmentSessionStatusPayload,
} from "@/types/assessment";

export async function listAssessmentSessions(): Promise<AssessmentSessionItem[]> {
  const response = await http.get("/api/assessment/sessions");
  return (response.data?.data?.items ?? []) as AssessmentSessionItem[];
}

export async function createAssessmentSession(payload: CreateAssessmentSessionPayload): Promise<AssessmentSessionDetail> {
  const response = await http.post("/api/assessment/sessions", payload);
  return response.data?.data as AssessmentSessionDetail;
}

export async function getAssessmentSession(sessionId: number): Promise<AssessmentSessionDetail> {
  const response = await http.get(`/api/assessment/sessions/${sessionId}`);
  return response.data?.data as AssessmentSessionDetail;
}

export async function updateAssessmentSession(
  sessionId: number,
  payload: UpdateAssessmentSessionPayload,
): Promise<AssessmentSessionDetail> {
  const response = await http.put(`/api/assessment/sessions/${sessionId}`, payload);
  return response.data?.data as AssessmentSessionDetail;
}

export async function updateAssessmentSessionStatus(
  sessionId: number,
  payload: UpdateAssessmentSessionStatusPayload,
): Promise<AssessmentSessionDetail> {
  const response = await http.put(`/api/assessment/sessions/${sessionId}/status`, payload);
  return response.data?.data as AssessmentSessionDetail;
}

export async function updateAssessmentPeriods(
  sessionId: number,
  payload: UpdateAssessmentPeriodsPayload,
) {
  const response = await http.put(`/api/assessment/sessions/${sessionId}/periods`, payload);
  return (response.data?.data?.items ?? []) as AssessmentSessionDetail["periods"];
}

export async function updateAssessmentObjectGroups(
  sessionId: number,
  payload: UpdateAssessmentObjectGroupsPayload,
) {
  const response = await http.put(`/api/assessment/sessions/${sessionId}/object-groups`, payload);
  return (response.data?.data?.items ?? []) as AssessmentSessionDetail["objectGroups"];
}

export async function listAssessmentSessionObjects(sessionId: number): Promise<AssessmentSessionObjectItem[]> {
  const response = await http.get(`/api/assessment/sessions/${sessionId}/objects`);
  return (response.data?.data?.items ?? []) as AssessmentSessionObjectItem[];
}

export async function listCalculatedAssessmentSessionObjects(
  sessionId: number,
  periodCode: string,
  objectGroupCode: string,
): Promise<AssessmentSessionObjectItem[]> {
  const response = await http.get(`/api/assessment/sessions/${sessionId}/calculated-objects`, {
    params: {
      periodCode: periodCode.trim().toUpperCase(),
      objectGroupCode: objectGroupCode.trim(),
    },
  });
  return (response.data?.data?.items ?? []) as AssessmentSessionObjectItem[];
}

export async function listAssessmentObjectCandidates(
  sessionId: number,
  keyword?: string,
): Promise<AssessmentObjectCandidateItem[]> {
  const response = await http.get(`/api/assessment/sessions/${sessionId}/object-candidates`, {
    params: {
      keyword: keyword?.trim() || undefined,
    },
  });
  return (response.data?.data?.items ?? []) as AssessmentObjectCandidateItem[];
}

export async function updateAssessmentObjects(
  sessionId: number,
  payload: UpdateAssessmentObjectsPayload,
): Promise<AssessmentSessionObjectItem[]> {
  const response = await http.put(`/api/assessment/sessions/${sessionId}/objects`, payload);
  return (response.data?.data?.items ?? []) as AssessmentSessionObjectItem[];
}

export async function resetAssessmentSessionObjects(sessionId: number): Promise<AssessmentSessionObjectItem[]> {
  const response = await http.post(`/api/assessment/sessions/${sessionId}/objects/reset-default`);
  return (response.data?.data?.items ?? []) as AssessmentSessionObjectItem[];
}

export async function upsertAssessmentModuleScores(
  sessionId: number,
  payload: UpdateAssessmentModuleScoresPayload,
): Promise<Array<{ periodCode: string; objectId: number; moduleKey: string; score: number }>> {
  const response = await http.put(`/api/assessment/sessions/${sessionId}/module-scores`, payload);
  return (response.data?.data?.items ?? []) as Array<{ periodCode: string; objectId: number; moduleKey: string; score: number }>;
}
