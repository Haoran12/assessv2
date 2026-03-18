import { http } from "@/api/http";
import type {
  CreateRuleFilePayload,
  RuleBindingItem,
  RuleFileItem,
  SelectRuleBindingPayload,
  UpdateRuleFilePayload,
} from "@/types/rules";

export async function listRuleFiles(assessmentId: number, includeHidden = false): Promise<RuleFileItem[]> {
  const response = await http.get("/api/rules/files", {
    params: {
      assessmentId,
      includeHidden,
    },
  });
  return (response.data?.data?.items ?? []) as RuleFileItem[];
}

export async function createRuleFile(payload: CreateRuleFilePayload): Promise<RuleFileItem> {
  const response = await http.post("/api/rules/files", payload);
  return response.data?.data as RuleFileItem;
}

export async function updateRuleFile(ruleId: number, payload: UpdateRuleFilePayload): Promise<RuleFileItem> {
  const response = await http.put(`/api/rules/files/${ruleId}`, payload);
  return response.data?.data as RuleFileItem;
}

export async function deleteRuleFile(ruleId: number): Promise<void> {
  await http.delete(`/api/rules/files/${ruleId}`);
}

export async function hideRuleFile(ruleId: number): Promise<void> {
  await http.post(`/api/rules/files/${ruleId}/hide`);
}

export async function unhideRuleFile(ruleId: number): Promise<void> {
  await http.delete(`/api/rules/files/${ruleId}/hide`);
}

export async function listRuleBindings(assessmentId: number, periodCode?: string): Promise<RuleBindingItem[]> {
  const response = await http.get("/api/rules/bindings", {
    params: {
      assessmentId,
      periodCode,
    },
  });
  return (response.data?.data?.items ?? []) as RuleBindingItem[];
}

export async function selectRuleBinding(payload: SelectRuleBindingPayload): Promise<RuleBindingItem> {
  const response = await http.post("/api/rules/bindings/select", payload);
  return response.data?.data as RuleBindingItem;
}
