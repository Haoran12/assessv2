import { http } from "@/api/http";
import type {
  AssessmentObjectItem,
  AssessmentPeriodItem,
  AssessmentPeriodStatus,
  AssessmentPeriodTemplateItem,
  AssessmentYearItem,
  AssessmentYearStatus,
  CreateAssessmentYearPayload,
  CreateAssessmentYearResult,
} from "@/types/assessment";

export async function listAssessmentYears(): Promise<AssessmentYearItem[]> {
  const response = await http.get("/api/assessment/years");
  return (response.data?.data?.items ?? []) as AssessmentYearItem[];
}

export async function createAssessmentYear(
  payload: CreateAssessmentYearPayload,
): Promise<CreateAssessmentYearResult> {
  const response = await http.post("/api/assessment/years", payload);
  return response.data?.data as CreateAssessmentYearResult;
}

export async function updateAssessmentYearStatus(
  yearId: number,
  status: AssessmentYearStatus,
): Promise<AssessmentYearItem> {
  const response = await http.put(`/api/assessment/years/${yearId}/status`, { status });
  return response.data?.data as AssessmentYearItem;
}

export async function listAssessmentPeriods(yearId: number): Promise<AssessmentPeriodItem[]> {
  const response = await http.get(`/api/assessment/years/${yearId}/periods`);
  return (response.data?.data?.items ?? []) as AssessmentPeriodItem[];
}

export async function listAssessmentPeriodTemplates(): Promise<AssessmentPeriodTemplateItem[]> {
  const response = await http.get("/api/assessment/period-templates");
  return (response.data?.data?.items ?? []) as AssessmentPeriodTemplateItem[];
}

export async function updateAssessmentPeriodTemplates(
  items: AssessmentPeriodTemplateItem[],
): Promise<AssessmentPeriodTemplateItem[]> {
  const response = await http.put("/api/assessment/period-templates", { items });
  return (response.data?.data?.items ?? []) as AssessmentPeriodTemplateItem[];
}

export async function updateAssessmentPeriodStatus(
  periodId: number,
  status: AssessmentPeriodStatus,
): Promise<AssessmentPeriodItem> {
  const response = await http.put(`/api/assessment/periods/${periodId}/status`, { status });
  return response.data?.data as AssessmentPeriodItem;
}

export async function listAssessmentObjects(yearId: number): Promise<AssessmentObjectItem[]> {
  const response = await http.get(`/api/assessment/years/${yearId}/objects`);
  return (response.data?.data?.items ?? []) as AssessmentObjectItem[];
}
